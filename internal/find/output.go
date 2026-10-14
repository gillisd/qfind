package find

import (
	"errors"
	"os"
	"sync"

	"golang.org/x/sys/unix"

	"qfind/internal/libc"
)

type streamKind uint8

const (
	streamStdout streamKind = iota
	streamStderr
	streamFile
)

// sink is a locked stdio stream: stdout, or a shared output file (Task 7).
// write and flush return this call's error, like the C call's return
// value; callers hold lock() around them.
type sink interface {
	lock()
	unlock()
	write(b []byte) error
	flush() error
}

// Stream is format_val's destination: FILE*, filename and dest_is_tty.
type Stream struct {
	name string // dest->filename: "standard output", "standard error", or the path as given
	kind streamKind
	tty  bool
	sink sink // nil for stderr, which is unbuffered and written at once
}

// Output owns stdout and, from Task 7, the shared output files.
type Output struct {
	r            *Run
	mu           sync.Mutex // guards f and closed
	f            *cfile
	closed       bool // closeAll has run: fd 1 is closed and writes are dropped
	stdoutStream *Stream
	stderrStream *Stream
}

func newOutput(r *Run) *Output {
	o := &Output{r: r, f: newCfile(os.Stdout)}
	o.stdoutStream = &Stream{name: "standard output", kind: streamStdout, tty: isatty(1), sink: o}
	o.stderrStream = &Stream{name: "standard error", kind: streamStderr, tty: isatty(2)}
	return o
}

func (o *Output) lock()   { o.mu.Lock() }
func (o *Output) unlock() { o.mu.Unlock() }

// write drops data once closeAll has run: a worker still committing after
// the exit claim must neither write to a closed (or reused) fd 1 nor report
// an error GNU, which dies at once, could never print.
func (o *Output) write(b []byte) error {
	if o.closed {
		return nil
	}
	return o.f.write(b)
}

func (o *Output) flush() error { return o.f.flushBuf() }

// failMode is what a failed write of a queued call does, following how
// GNU checks the stdio call that produced it.
type failMode uint8

const (
	failIgnored  failMode = iota // print_quoted, fputs, putc: unchecked (-print, -print0, -fprint)
	failReported                 // checked_fprintf/fwrite/fflush: diagnose, exit status 1 (-printf)
	failFatal                    // list_file: "Failed to write output (at stage N)" (-ls)
)

// piece is one stdio call of a rendered record; stage is list_file's
// failed_at number for that call.
type piece struct {
	text  string
	stage int
}

// pending is one queued stdio call, or (render) one -ls record whose calls
// are produced at commit time: listfile.c's column widths are shared by
// every entry and must grow in output order.
type pending struct {
	s      *Stream
	text   string
	fail   failMode
	render func() []piece
	done   bool
}

// txn is one entry's pending output. Committing it atomically keeps an
// entry's records together, as GNU's one-entry-at-a-time evaluation does.
type txn struct{ items []pending }

// emit queues one unchecked call (print_quoted, fputs) for s on e.
func (r *Run) emit(e *Entry, s *Stream, text string) { r.emitChecked(e, s, text, failIgnored) }

// emitChecked queues one call whose failure is handled per fail. stderr
// is unbuffered in C, so its text is written immediately, in program order
// with diagnostics.
func (r *Run) emitChecked(e *Entry, s *Stream, text string, fail failMode) {
	if s.sink == nil {
		if err := r.writeStderr(text); err != nil && fail == failReported {
			r.reportWriteError(nil, s, err)
		}
		return
	}
	e.txn.items = append(e.txn.items, pending{s: s, text: text, fail: fail})
}

// emitRendered queues a record whose calls are produced at commit time.
func (r *Run) emitRendered(e *Entry, s *Stream, render func() []piece) {
	if s.sink == nil {
		for _, pc := range render() {
			if err := r.writeStderr(pc.text); err != nil {
				r.fatalf(err, "Failed to write output (at stage %d)", pc.stage)
			}
		}
		return
	}
	e.txn.items = append(e.txn.items, pending{s: s, fail: failFatal, render: render})
}

// emitQuoted is lib/printquoted.c print_quoted(fp, …, "%s<suffix>", name).
func (r *Run) emitQuoted(e *Entry, s *Stream, name, suffix string) {
	if s.tty {
		name = r.loc.Qmark(name)
	}
	r.emit(e, s, name+suffix)
}

// commit hands e's queued calls to their streams. Each sink receives all
// of e's calls for it, in order, under one lock, so no other entry's
// record lands between two of e's; the order between sinks is not
// observable.
func (r *Run) commit(e *Entry) {
	items := e.txn.items
	for i := range items {
		if items[i].done {
			continue
		}
		sk := items[i].s.sink
		sk.lock()
		for j := i; j < len(items); j++ {
			it := &items[j]
			if it.done || it.s.sink != sk {
				continue
			}
			it.done = true
			if it.render != nil {
				for _, pc := range it.render() {
					if err := sk.write([]byte(pc.text)); err != nil {
						sk.unlock() // exit() flushes every stream: never die holding one
						r.fatalf(err, "Failed to write output (at stage %d)", pc.stage)
					}
				}
				continue
			}
			if err := sk.write([]byte(it.text)); err != nil && it.fail == failReported {
				r.reportWriteError(sk, it.s, err)
			}
		}
		sk.unlock()
	}
	e.txn.items = items[:0]
}

// reportWriteError is nonfatal_nontarget_file_error(errno, dest->filename)
// for a checked call, issued while sink held (or none) is locked. Like
// error(), it flushes stdout first; stdout's lock may be the one held.
func (r *Run) reportWriteError(held sink, s *Stream, err error) {
	o := r.out
	if held != sink(o) {
		o.mu.Lock()
		defer o.mu.Unlock()
	}
	o.f.flushBuf()
	r.writeStderr(r.prog + ": " + r.quote(s.name) + ": " + libc.Strerror(err) + "\n")
	r.setFailure()
}

// flushStream is fflush(fp): unchecked at launch() and the -ok prompt
// (stdout), checked_fflush at \c (the format's own stream).
func (r *Run) flushStream(e *Entry, s *Stream, fail failMode) {
	if e != nil {
		r.commit(e)
	}
	if s.sink == nil {
		return
	}
	s.sink.lock()
	defer s.sink.unlock()
	if err := s.sink.flush(); err != nil && fail == failReported {
		r.reportWriteError(s.sink, s, err)
	}
}

// flushStdout is the unchecked fflush(stdout) before a child or a prompt.
func (r *Run) flushStdout(e *Entry) { r.flushStream(e, r.out.stdoutStream, failIgnored) }

// fatalAt is fatalf for an error raised while evaluating e. GNU's stdout
// buffer already holds e's earlier output and exit() flushes it, so commit
// e's transaction first.
func (r *Run) fatalAt(e *Entry, err error, format string, a ...any) {
	if e != nil {
		r.commit(e)
	}
	r.fatalf(err, format, a...)
}

// errorAt is errorf for a non-fatal diagnostic raised while considering or
// evaluating e. glibc error() flushes stdout before printing, and GNU's
// stdout buffer already holds e's earlier output, so commit e's
// transaction first: in a merged stream the message follows that output.
func (r *Run) errorAt(e *Entry, err error, format string, a ...any) {
	if e != nil {
		r.commit(e)
	}
	r.errorf(err, format, a...)
}

// flushForDiagnostic is the fflush(stdout) inside glibc error().
func (o *Output) flushForDiagnostic() {
	o.mu.Lock()
	o.f.flushBuf()
	o.mu.Unlock()
}

// flushFilesQuietly is exit()'s _IO_cleanup flush of every FILE still
// open: errors are ignored here; only fclose in cleanup reports them.
func (o *Output) flushFilesQuietly() {
	reg := &o.r.shared
	reg.mu.Lock()
	files := reg.order.all()
	reg.mu.Unlock()
	for _, sf := range files {
		sf.lock()
		if !sf.closed {
			sf.flush()
		}
		sf.unlock()
	}
}

// WriteString writes usage, version and -D help text (no entry).
func (o *Output) WriteString(s string) {
	o.mu.Lock()
	o.write([]byte(s))
	o.mu.Unlock()
}

var errPrevFailed = errors.New("write error after earlier failure")

// cleanupFlush is util.c cleanup()'s fflush(stdout): it fails only when
// this flush's write fails; a buffer emptied by an earlier failure flushes
// fine, and close_stdout reports that one.
func (r *Run) cleanupFlush() {
	o := r.out
	o.mu.Lock()
	err := o.f.flushBuf()
	o.mu.Unlock()
	if err != nil {
		r.errorf(err, "%s", r.quote("standard output"))
		r.setFailure()
	}
}

// closeAll is close_stdout's close_stream(stdout): fclose flushes and
// closes fd 1. An earlier failure gives "write error" without errno, a
// failure now gives its errno, and EBADF with nothing pending (stdout
// closed but never written) is no error. Later writes are dropped.
func (o *Output) closeAll() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.closed = true
	pending := len(o.f.buf) > 0
	prevFail := o.f.err != nil
	err := o.f.flushBuf()
	if cerr := unix.Close(1); err == nil {
		err = cerr
	}
	switch {
	case err == nil && prevFail:
		return errPrevFailed
	case err != nil && (prevFail || pending || err != unix.EBADF):
		return err
	}
	return nil
}

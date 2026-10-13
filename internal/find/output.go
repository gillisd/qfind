package find

import (
	"bufio"
	"errors"
	"os"
	"sync"
	"syscall"
)

// Output owns stdout. This minimal version only carries the usage, version
// and -D help texts through one stdio-like buffer; Task 3 replaces it with
// per-entry output transactions and keeps these method names.
type Output struct {
	mu     sync.Mutex
	w      *bufio.Writer
	failed bool // ferror(stdout): a write to fd 1 has failed
}

// newOutput buffers stdout; Task 3's version keeps r for its diagnostics.
func newOutput(r *Run) *Output {
	return &Output{w: bufio.NewWriterSize(os.Stdout, 64<<10)}
}

// WriteString writes usage, version and -D help text (no entry).
func (o *Output) WriteString(s string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, err := o.w.WriteString(s); err != nil {
		o.fail()
	}
}

// fail records a failed write as glibc does: the stream's error flag stays
// set and the buffered bytes are dropped (libio new_do_write empties the
// buffer whether or not the write succeeded).
func (o *Output) fail() {
	o.failed = true
	o.w.Reset(os.Stdout)
}

// flushForDiagnostic is the fflush(stdout) inside glibc error().
func (o *Output) flushForDiagnostic() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if err := o.w.Flush(); err != nil {
		o.fail()
	}
}

// flushFilesQuietly is exit()'s flush of every other FILE; Task 7 fills it.
func (o *Output) flushFilesQuietly() {}

var errPrevFailed = errors.New("write error after earlier failure")

// closeAll is close_stdout's close_stream(stdout): fclose flushes and
// closes fd 1. An earlier failure gives "write error" without errno, a
// failure now gives its errno, and EBADF with nothing pending (stdout
// closed but never written) is no error.
func (o *Output) closeAll() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	pending := o.w.Buffered() > 0
	err := o.w.Flush()
	if cerr := os.Stdout.Close(); err == nil {
		err = cerr
	}
	switch {
	case err == nil && o.failed:
		return errPrevFailed
	case err != nil && (o.failed || pending || !errors.Is(err, syscall.EBADF)):
		return err
	}
	return nil
}

package find

import (
	"fmt"
	"os"
	"strings"

	"qfind/internal/libc"
)

// errorf is glibc error(0, errnum, format, …): flush stdout, then print
// "<prog>: <message>[: <strerror>]\n" on stderr. It does not set the exit
// status; callers do, as GNU does.
func (r *Run) errorf(err error, format string, a ...any) {
	msg := r.errorText(err, format, a...)
	if r.out == nil {
		r.writeStderr(msg)
		return
	}
	r.out.diagnose(msg)
}

// errorText is the line error() prints.
func (r *Run) errorText(err error, format string, a ...any) string {
	var b strings.Builder
	b.WriteString(r.prog)
	b.WriteString(": ")
	fmt.Fprintf(&b, format, a...)
	if err != nil {
		b.WriteString(": ")
		b.WriteString(libc.Strerror(err))
	}
	b.WriteByte('\n')
	return b.String()
}

// writeStderr is an unbuffered fputs to stderr: one write per record, and
// a failure is remembered like ferror(stderr).
func (r *Run) writeStderr(text string) error {
	r.stderrMu.Lock()
	defer r.stderrMu.Unlock()
	_, err := os.Stderr.WriteString(text)
	if err != nil {
		r.stderrFailed.Store(true)
	}
	return err
}

// debugf is fprintf(stderr, …) for -D output: no program-name prefix. Like
// fprintf, it makes no write(2) when the text is empty.
func (r *Run) debugf(format string, a ...any) {
	if s := fmt.Sprintf(format, a...); s != "" {
		r.writeStderr(s)
	}
}

// cbyte is C's %c of a char: the raw byte. Go's %c would UTF-8-encode a
// byte >= 0x80 as a rune, which GNU's messages never do.
func cbyte(b byte) string { return string([]byte{b}) }

// fatalf is error(EXIT_FAILURE, errnum, …): report, then exit(1) through the
// same flush-and-close path as a normal exit. It claims the exit before it
// reports, so only the first fatal error is printed.
func (r *Run) fatalf(err error, format string, a ...any) {
	r.claimExit()
	r.errorf(err, format, a...)
	r.exitClaimed(1)
}

// usageFailure is usage(EXIT_FAILURE): no program prefix on the hint.
func (r *Run) usageFailure() {
	r.writeStderr(fmt.Sprintf("Try '%s --help' for more information.\n", r.prog))
	r.exit(1)
}

// exit is exit(status) with gnulib close_stdout() registered via atexit:
// stdout is closed, then stdio flushes the shared output files; a stdout
// failure prints "write error" and exits 1 at once (EPIPE included: GNU sets no
// ignore_EPIPE, and SIGPIPE normally kills the process first).
//
// exit is the only way out of the process; see claimExit.
func (r *Run) exit(status int) {
	r.claimExit()
	r.exitClaimed(status)
}

// claimExit makes the caller the one goroutine that exits the process (GNU,
// being single-threaded, dies once). The first caller wins. A later caller
// is on another goroutine, because nothing that runs after a claim dies
// again, so it blocks until the winner's os.Exit instead of printing a
// second message or exiting with another status. A write failure inside
// fatalAt's commit claims first, as GNU would have died at that write.
// A parked loser keeps any locks it holds forever, so exit and fatalf must
// never be called while holding stdout's lock: the winner's closeAll needs
// it (commit unlocks its sink before fatalf).
func (r *Run) claimExit() {
	if !r.exiting.CompareAndSwap(false, true) {
		select {}
	}
}

// BeforeExit, when set, runs just before the process exits; cmd/qfind uses
// it to stop a QFIND_CPUPROFILE profile, which os.Exit would truncate.
var BeforeExit func()

// osExit is os.Exit after the BeforeExit hook.
func osExit(status int) {
	if BeforeExit != nil {
		BeforeExit()
	}
	os.Exit(status)
}

// exitClaimed is exit's work once the caller holds the claim.
func (r *Run) exitClaimed(status int) {
	r.drainBatch() // batched records were already written, in GNU's terms
	// close_stdout runs first, from atexit; a stdout failure _exit(1)s,
	// which skips stdio's flush of the other FILEs (gl/lib/closeout.c).
	if r.out != nil {
		if err := r.out.closeAll(); err != nil {
			if err == errPrevFailed {
				r.errorf(nil, "write error")
			} else {
				r.errorf(err, "write error")
			}
			osExit(1)
		}
	}
	// close_stdout also runs close_stream(stderr): after any failed write to
	// stderr (2>/dev/full), it calls _exit(EXIT_FAILURE) without a message.
	if r.stderrFailed.Load() {
		osExit(1)
	}
	if r.out != nil {
		r.out.flushFilesQuietly() // exit()'s _IO_cleanup flushes every other FILE
	}
	osExit(status)
}

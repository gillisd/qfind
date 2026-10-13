package find

import (
	"os"
	"sync"
	"sync/atomic"
	"time"

	"qfind/internal/libc"
)

// Run is the process-wide state: GNU's `options` plus the parts of `state`
// that are not per file (exit status, shared files, pending execs).
type Run struct {
	prog         string // argv[0], used like program_invocation_name
	opt          Options
	loc          libc.Locale
	tz           *time.Location
	env          []string
	out          *Output // Task 3
	stderrMu     sync.Mutex
	stderrFailed atomic.Bool // ferror(stderr): close_stdout then exits 1
	exitStatus   atomic.Int32
	exiting      atomic.Bool // set by the one goroutine that exits; see claimExit
}

func newRun(args []string) *Run {
	prog := "find"
	if len(args) > 0 {
		prog = args[0]
	}
	r := &Run{prog: prog, env: os.Environ()}
	r.loc = libc.LocaleFromEnv(r.lookupEnv)
	r.tz = libc.LoadTZ(r.lookupEnv)
	return r
}

func (r *Run) lookupEnv(name string) (string, bool) { return os.LookupEnv(name) }

func (r *Run) setFailure() { r.exitStatus.Store(1) }

// quote is safely_quote_err_filename: locale_quoting_style.
func (r *Run) quote(s string) string { return r.loc.Quote(s) }

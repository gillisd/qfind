package find

import (
	"bufio"
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
	out          *Output       // Task 3
	shared       shareRegistry // state.shared_files (Task 7)
	stderrMu     sync.Mutex
	stderrFailed atomic.Bool // ferror(stderr): close_stdout then exits 1
	exitStatus   atomic.Int32
	exiting      atomic.Bool // set by the one goroutine that exits; see claimExit
	countRates   bool        // -D rates: apply counts visits and successes
	startPoints  []string    // the command-line starting points (tree.c start_points)
	tree         *Predicate  // the evaluation tree (eval_tree)
	preds        *Predicate  // the command-line predicate list (predicates)

	// Traversal (Task 4). GNU considers one entry at a time; qfind does so
	// only when that is observable across entries (needsSerial), holding
	// evalMu around consider_visiting and every visit.
	serial        bool
	evalMu        sync.Mutex
	workers       int          // fastwalk's NumWorkers
	prevDepth     atomic.Int64 // ftsfind.c prev_depth, for -D search
	formatFlushes bool         // a format contains \c (Task 7)
	slot0         quoteSlot0

	// Command execution (Task 8).
	execMu       sync.Mutex    // one child at a time
	stdin        *bufio.Reader // find's stdin, read by the -ok question
	inheritedFDs map[int]bool  // non-close-on-exec descriptors at startup
	heldFDs      []int         // -samefile reference files, held open for the run (parse_samefile)

	lsw *lsWidths // listfile.c's static column widths (-ls, -fls; set by insertFls)
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

// quote is safely_quote_err_filename(0, s): locale_quoting_style in
// quotearg slot 0, which it also records (see slot0).
func (r *Run) quote(s string) string {
	q := r.loc.Quote(s)
	r.slot0.store(q)
	return q
}

// quoteN is quotearg_n_style(n, locale_quoting_style, s) for a slot n > 0.
func (r *Run) quoteN(s string) string { return r.loc.Quote(s) }

// quoteSlot0 is gnulib quotearg's static slot0 buffer. ftsfind.c keeps a
// pointer into it as files0_filename_quoted, so its -files0-from messages
// name whatever slot 0 quoted last (4.11.0 bug). A quote of 256 bytes or
// more moves slot 0 to the heap for good; its first pass has filled the
// static buffer with the quote's first 256 bytes and no terminator (the
// oracle then reads a zero byte after it), and that buffer stays frozen.
type quoteSlot0 struct {
	mu     sync.Mutex
	s      string
	frozen bool
}

func (q *quoteSlot0) store(s string) {
	q.mu.Lock()
	if !q.frozen {
		q.s = s[:min(len(s), 256)]
		q.frozen = len(s) >= 256
	}
	q.mu.Unlock()
}

func (q *quoteSlot0) load() string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.s
}

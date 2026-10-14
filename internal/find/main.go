// Package find is a port of GNU findutils 4.11.0 find/*.c with fts
// replaced by github.com/charlievieth/fastwalk.
package find

import (
	"math"
	"time"

	"qfind/internal/libc"
)

// Main runs find with argv and returns the exit status (ftsfind.c main).
func Main(args []string) int {
	r := newRun(args)
	r.recordInheritedFDs()
	r.out = newOutput(r)
	r.setDefaults()
	end := r.processLeadingOptions(args)
	if r.opt.Debug&DebugTime != 0 {
		r.writeStderr("cur_day_start = " + libcCtime(r))
	}
	r.tree = r.buildExpressionTree(args, end)
	r.countRates = r.opt.Debug&DebugSuccessRates != 0
	r.workers = r.numWorkers()
	r.prevDepth.Store(math.MinInt32)
	r.serial = r.needsSerial()
	if r.processAllStartPoints() {
		r.showSuccessRates(r.tree)
		r.cleanup()
	}
	r.exit(int(r.exitStatus.Load()))
	return 0
}

// needsSerial reports whether GNU's one-entry-at-a-time evaluation is
// observable across entries for this command line: exec-family actions
// share stdin and flush stdout before each child, -quit must stop
// everything at once, \c flushes mid-record, and -D narrates global state.
func (r *Run) needsSerial() bool {
	if r.opt.Debug != 0 || r.formatFlushes {
		return true
	}
	for p := r.preds; p != nil; p = p.next {
		switch p.id {
		case fnQuit, fnExec, fnExecdir, fnOk, fnOkdir:
			return true
		}
	}
	return false
}

func libcCtime(r *Run) string { return libc.Ctime(time.Unix(r.opt.CurDayStart.Sec, 0).In(r.tz)) }

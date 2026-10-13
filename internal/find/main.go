// Package find is a port of GNU findutils 4.11.0 find/*.c with fts
// replaced by github.com/charlievieth/fastwalk.
package find

import (
	"time"

	"qfind/internal/libc"
)

// Main runs find with argv and returns the exit status (ftsfind.c main).
func Main(args []string) int {
	r := newRun(args)
	r.out = newOutput(r)
	r.setDefaults()
	end := r.processLeadingOptions(args)
	if r.opt.Debug&DebugTime != 0 {
		r.writeStderr("cur_day_start = " + libcCtime(r))
	}
	r.tree = r.buildExpressionTree(args, end)
	r.countRates = r.opt.Debug&DebugSuccessRates != 0
	if r.processAllStartPoints() {
		r.showSuccessRates(r.tree)
		r.cleanup()
	}
	r.exit(int(r.exitStatus.Load()))
	return 0
}

func libcCtime(r *Run) string { return libc.Ctime(time.Unix(r.opt.CurDayStart.Sec, 0).In(r.tz)) }

// Package find is a port of GNU findutils 4.11.0 find/*.c with fts
// replaced by github.com/charlievieth/fastwalk.
package find

import (
	"strings"
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
	// Task 3: build the expression tree (parses -help/-version, which exit).
	// Task 4: process start points, show success rates, cleanup.
	switch helpShim(args, end) {
	case "help":
		r.usage()
	case "version":
		r.version()
	}
	r.exit(int(r.exitStatus.Load()))
	return 0
}

func libcCtime(r *Run) string { return libc.Ctime(time.Unix(r.opt.CurDayStart.Sec, 0).In(r.tz)) }

// helpShim stands in for Task 3's parse table, which deletes it. It returns
// "help" or "version" when the first argument after the start points is
// -help, --help, -version or --version (find_parser strips one dash before
// its lookup), the only position where GNU parses that primary before any
// other.
func helpShim(args []string, end int) string {
	for _, a := range args[min(end, len(args)):] {
		// util.c looks_like_expression(a, true) ends the start points.
		if !(len(a) > 1 && a[0] == '-' || a == "!" || a == "(") {
			continue
		}
		switch strings.TrimPrefix(a, "-") {
		case "help", "-help":
			return "help"
		case "version", "-version":
			return "version"
		}
		return ""
	}
	return ""
}

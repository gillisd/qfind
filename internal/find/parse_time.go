package find

import (
	"math"
	"strings"
	"time"

	"qfind/internal/datetime"
	"qfind/internal/libc"
)

const daySecs = 86400

// xval is enum xval (defs.h): which timestamp -newerXY compares.
type xval uint8

const (
	xvalAtime xval = iota
	xvalBirthtime
	xvalCtime
	xvalMtime
)

// timeVal is struct time_val (defs.h).
type timeVal struct {
	xval xval
	ts   Timespec
	kind compKind
}

// cmp is gnulib timespec_cmp: compare seconds, then nanoseconds.
func (t Timespec) cmp(u Timespec) int {
	switch {
	case t.Sec != u.Sec:
		if t.Sec < u.Sec {
			return -1
		}
		return 1
	case t.Nsec < u.Nsec:
		return -1
	case t.Nsec > u.Nsec:
		return 1
	}
	return 0
}

// estimateFileAgeSuccessRate is parser.c estimate_file_age_success_rate.
func estimateFileAgeSuccessRate(numDays float32) float32 {
	switch {
	case numDays < 0.1:
		return 0.01
	case numDays < 1.0:
		return 0.3
	case numDays > 100.0:
		return 0.3
	}
	return 0.39
}

// estimateTimestampSuccessRate is parser.c estimate_timestamp_success_rate
// (int num_days = (cur_day_start - when) / 86400).
func (r *Run) estimateTimestampSuccessRate(when int64) float32 {
	return estimateFileAgeSuccessRate(float32(int32((r.opt.CurDayStart.Sec - when) / daySecs)))
}

// relativeTimestamp is parser.c get_relative_timestamp: the comparison is
// inverted (older than N means a timestamp less than origin-N), the count
// is a C double, and the subtraction mirrors C's double-to-time_t
// conversion (the same instruction on each platform as in C).
func (r *Run) relativeTimestamp(str string, origin Timespec, secPerUnit float64, overflowMsg string) (timeVal, bool) {
	var tv timeVal
	kind, rest := getCompType(str)
	switch kind {
	case compLT:
		kind = compGT
	case compGT:
		kind = compLT
	}
	tv.kind = kind
	offset, ok := libc.Xstrtod(rest)
	if !ok {
		return tv, false
	}
	if math.IsNaN(offset) {
		r.fatalf(nil, "invalid not-a-number argument: `%s'", rest)
	}
	seconds, frac := cModf(offset * secPerUnit)
	nanosec := frac * 1e9
	sec := int64(float64(origin.Sec) - seconds)
	if (origin.Sec < sec) != (seconds < 0) {
		r.fatalf(nil, overflowMsg, rest)
	}
	nsec := int64(float64(origin.Nsec) - nanosec)
	if float64(origin.Nsec) < nanosec {
		nsec += 1e9
		sec--
	}
	tv.ts = Timespec{sec, nsec}
	return tv, true
}

// cModf is C modf: unlike math.Modf, an infinity has a zero fraction.
func cModf(f float64) (float64, float64) {
	if math.IsInf(f, 0) {
		return f, math.Copysign(0, f)
	}
	return math.Modf(f)
}

// ctimeC is ctime(3), including its NULL (printed by glibc as "(null)")
// when the year does not fit asctime's int.
func (r *Run) ctimeC(t int64) string {
	tm := time.Unix(t, 0).In(r.tz)
	if y := int64(tm.Year()) - 1900; y < math.MinInt32 || y > math.MaxInt32-1900 {
		return "(null)"
	}
	return libc.Ctime(tm)
}

// parseTime is parser.c parse_time (-atime, -ctime, -mtime).
func parseTime(ps *parser, e *parserEntry) bool {
	r := ps.r
	saved := ps.i
	timearg, ok := ps.collectArg()
	if !ok {
		return false
	}
	origin := r.opt.CurDayStart
	if kind, _ := getCompType(timearg); kind == compLT {
		// GNU's "end of today" overflow check compares the wrapped sum
		// with itself, so it never fires: the addition just wraps.
		origin.Sec += daySecs - 1
	}
	tv, ok := r.relativeTimestamp(timearg, origin, daySecs, "arithmetic overflow while converting %s days to a number of seconds")
	if !ok {
		ps.i = saved // don't consume the invalid argument
		return false
	}
	p := ps.insertPrimary(e, timearg, true)
	p.args.reftime = tv
	p.estSuccessRate = r.estimateTimestampSuccessRate(tv.ts.Sec)
	if r.opt.Debug&DebugExpressionTree != 0 {
		names := [...][2]string{compGT: {"gt", " >"}, compLT: {"lt", " <"}, compEQ: {"eq", ">="}}
		r.debugf("inserting %s\n    type: %s    %s  %d %s", p.name, names[tv.kind][0], names[tv.kind][1],
			uint64(tv.ts.Sec), r.ctimeC(tv.ts.Sec))
		if tv.kind == compEQ {
			t := tv.ts.Sec + daySecs
			r.debugf("                 <  %d %s", uint64(t), r.ctimeC(t))
		}
	}
	return true
}

// parseXmin is parser.c do_parse_xmin (-amin, -cmin, -mmin): minutes back
// from the end of the reference day (cur_day_start + 1 day).
func parseXmin(ps *parser, e *parserEntry) bool {
	r := ps.r
	saved := ps.i
	minutes, ok := ps.collectArg()
	if !ok {
		return false
	}
	origin := r.opt.CurDayStart
	origin.Sec += daySecs
	tv, ok := r.relativeTimestamp(minutes, origin, 60, "arithmetic overflow while converting %s minutes to a number of seconds")
	if !ok {
		ps.i = saved
		return false
	}
	p := ps.insertPrimary(e, minutes, true)
	p.args.reftime = tv
	p.estSuccessRate = r.estimateTimestampSuccessRate(tv.ts.Sec)
	return true
}

// parseUsed is parser.c parse_used: a delta from 0, in days.
func parseUsed(ps *parser, e *parserEntry) bool {
	r := ps.r
	offset, ok := ps.collectArg()
	if !ok {
		return false
	}
	tv, ok := r.relativeTimestamp(offset, Timespec{}, daySecs, "arithmetic overflow while converting %s days to a number of seconds")
	if !ok {
		r.fatalf(nil, "Invalid argument %s to -used", offset)
	}
	p := ps.insertPrimary(e, offset, true)
	p.args.reftime = tv
	p.estSuccessRate = estimateFileAgeSuccessRate(float32(tv.ts.Sec / daySecs))
	return true
}

// insertNewer is parse_newer/parse_anewer/parse_cnewer: the reference's
// mtime (for all three) against the file's x timestamp.
func (ps *parser) insertNewer(e *parserEntry, x xval) bool {
	name, st, ok := ps.collectArgStatInfo()
	if !ok {
		return false
	}
	p := ps.insertPrimary(e, name, true)
	p.args.reftime = timeVal{xval: x, ts: tsOf(st.Mtim), kind: compGT}
	p.estSuccessRate = ps.r.estimateTimestampSuccessRate(st.Mtim.Sec)
	return true
}

func parseNewer(ps *parser, e *parserEntry) bool  { return ps.insertNewer(e, xvalMtime) }
func parseAnewer(ps *parser, e *parserEntry) bool { return ps.insertNewer(e, xvalAtime) }
func parseCnewer(ps *parser, e *parserEntry) bool { return ps.insertNewer(e, xvalCtime) }

// parseNewerXY is parser.c parse_newerXY. It is ARG_SPECIAL_PARSE: ps.i is
// at the -newerXY word itself. This build has no birth times.
func parseNewerXY(ps *parser, e *parserEntry) bool {
	r := ps.r
	if ps.i >= len(ps.argv) || len(ps.argv[ps.i]) != 8 {
		return false
	}
	word := ps.argv[ps.i]
	x, y := word[6], word[7]
	if x == 'B' || y == 'B' {
		r.errorf(nil, "This system does not provide a way to find the birth time of a file.")
		return false
	}
	const valid = "aBcmt"
	if x == 't' || strings.IndexByte(valid, x) < 0 || strings.IndexByte(valid, y) < 0 {
		return false
	}
	if ps.i+1 >= len(ps.argv) {
		r.fatalf(nil, "The %s test needs to be followed by an argument", r.quote(word))
	}
	ps.i++
	arg := ps.argv[ps.i]
	p := ps.insertPrimary(e, arg, true)
	tv := timeVal{kind: compGT}
	switch x {
	case 'a':
		tv.xval = xvalAtime
	case 'c':
		tv.xval = xvalCtime
	default:
		tv.xval = xvalMtime
	}
	if y == 't' {
		ts, ok := datetime.Parse(arg, datetime.Timespec(r.opt.StartTime), r.tz)
		if !ok {
			r.fatalf(nil, "I cannot figure out how to interpret %s as a date or time", r.quote(arg))
		}
		tv.ts = Timespec(ts)
	} else {
		st, err := r.xstatName(arg, arg, 0)
		if err != nil {
			r.fatalTargetFileError(err, arg)
		}
		switch y {
		case 'a':
			tv.ts = tsOf(st.Atim)
		case 'c':
			tv.ts = tsOf(st.Ctim)
		default:
			tv.ts = tsOf(st.Mtim)
		}
	}
	p.args.reftime = tv
	p.estSuccessRate = r.estimateTimestampSuccessRate(tv.ts.Sec)
	ps.i++
	return true
}

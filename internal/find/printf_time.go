package find

import (
	"fmt"
	"strconv"
	"time"

	"qfind/internal/libc"
)

var (
	weekdays = [...]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	months   = [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
)

// localtime is localtime(3) in find's zone; false when the year does not
// fit a C int (glibc fails with EOVERFLOW).
func (r *Run) localtime(sec int64) (time.Time, bool) {
	t := time.Unix(sec, 0).In(r.tz)
	y := int64(t.Year()) - 1900
	return t, y == int64(int32(y))
}

// ctimeFormat is print.c ctime_format: "%3s %3s %2d %02d:%02d:%02d.%09ld0 %04d",
// or the %@ form when localtime fails.
func (r *Run) ctimeFormat(ts Timespec) string {
	t, ok := r.localtime(ts.Sec)
	if !ok {
		return r.formatDate(ts, '@')
	}
	return fmt.Sprintf("%3s %3s %2d %02d:%02d:%02d.%09d0 %04d", weekdays[t.Weekday()],
		months[t.Month()-1], t.Day(), t.Hour(), t.Minute(), t.Second(), ts.Nsec, t.Year())
}

// formatDate is print.c format_date. The nanosecond suffix is
// ".%09ld0"; for the absent birth time {-1,-1} that is ".-000000010", as
// in C (Go's %09d pads a negative number the same way).
func (r *Run) formatDate(ts Timespec, kind byte) string {
	format := "%" + string(kind)
	needNS := kind == 'S' || kind == 'T' || kind == 'X' || kind == '@'
	if kind == '+' {
		format, needNS = "%Y-%m-%d+%T", true // GNU avoids %F
	}
	ns := ""
	if needNS {
		ns = fmt.Sprintf(".%09d0", ts.Nsec)
	}
	if kind != '@' {
		if t, ok := r.localtime(ts.Sec); ok {
			return doTimeFormat(format, t, ns)
		}
	}
	// %@, or localtime failed: human_readable of the magnitude, signed.
	if ts.Sec < 0 {
		return "-" + strconv.FormatUint(uint64(-(ts.Sec+1))+1, 10) + ns
	}
	return strconv.FormatUint(uint64(ts.Sec), 10) + ns
}

// doTimeFormat is print.c do_time_format: strftime "_"+fmt, then again
// with tm_sec ± 11. If exactly one run of 2 differing digits followed by a
// non-digit is found, the nanoseconds are inserted after it.
//
// Changing tm_sec by ±11 in C keeps every other field; time.Add could
// roll the minute over only if the seconds left 0–59, which ±11 prevents.
func doTimeFormat(format string, t time.Time, ns string) string {
	buf := libc.Strftime("_"+format, t)
	alt := t.Add(-11 * time.Second)
	if t.Second() < 11 {
		alt = t.Add(11 * time.Second)
	}
	altBuf := libc.Strftime("_"+format, alt)
	if i, n, ok := digitDiff(buf, altBuf); ok && n == 2 && (i+n >= len(buf) || !isDigit(buf[i+n])) {
		buf = buf[:i+n] + ns + buf[i+n:]
	}
	return buf[1:]
}

// digitDiff is print.c scan_for_digit_differences.
func digitDiff(p, q string) (first, n int, ok bool) {
	if len(p) != len(q) {
		return 0, 0, false
	}
	seen := false
	for i := 0; i < len(p); i++ {
		if p[i] == q[i] {
			continue
		}
		if !isDigit(p[i]) || !isDigit(q[i]) {
			return 0, 0, false
		}
		switch {
		case !seen:
			first, n, seen = i, 1, true
		case i-first == n:
			n++
		default:
			return 0, 0, false
		}
	}
	return first, n, true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

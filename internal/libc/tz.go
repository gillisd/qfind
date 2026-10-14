package libc

import (
	"encoding/binary"
	"os"
	"strings"
	"sync"
	"time"
)

// LoadTZ resolves $TZ like glibc tzset(): unset → /etc/localtime (Go's
// time.Local); otherwise TZFromString's rules.
func LoadTZ(lookup func(string) (string, bool)) *time.Location {
	tz, ok := lookup("TZ")
	if !ok {
		return time.Local
	}
	return tzFromString(tz, lookup)
}

// TZFromString is the zone glibc tzset() makes of a set TZ value (gnulib
// tzalloc(tz), as parse_datetime's TZ="..." prefix uses it): empty → UTC;
// ":name" or "name" → zoneinfo file under $TZDIR; otherwise a POSIX rule
// string such as "EST5" or "JST-9", which Go's time package silently turns
// into UTC.
func TZFromString(tz string) *time.Location {
	return tzFromString(tz, os.LookupEnv)
}

func tzFromString(tz string, lookup func(string) (string, bool)) *time.Location {
	name := strings.TrimPrefix(tz, ":")
	if name == "" {
		return time.UTC
	}
	if data, err := os.ReadFile(tzfilePath(name, lookup)); err == nil {
		if loc, err := time.LoadLocationFromTZData(name, data); err == nil {
			return loc
		}
	}
	if loc := posixZone(name); loc != nil {
		posixRules.Store(loc, true)
		return loc
	}
	return time.UTC
}

// posixRules records the locations made from POSIX rule strings.
var posixRules sync.Map

// IsPOSIXRule reports whether loc came from a POSIX rule string rather
// than a zoneinfo file. glibc evaluates such a zone without a tzfile, and
// its localtime then fails (EOVERFLOW) when the UTC year does not fit an
// int (tzset.c __tz_convert's __offtime(timer, 0, tp)), even if the local
// year would.
func IsPOSIXRule(loc *time.Location) bool {
	_, ok := posixRules.Load(loc)
	return ok
}

// tzfilePath is the file glibc's __tzfile_read opens for name: an absolute
// name as it is, any other under $TZDIR or /usr/share/zoneinfo.
// time.LoadLocation is not used because it rejects absolute names (as in
// TZ=:/etc/localtime) and takes "Local" to mean the system zone.
func tzfilePath(name string, lookup func(string) (string, bool)) string {
	if strings.HasPrefix(name, "/") {
		return name
	}
	dir, ok := lookup("TZDIR")
	if !ok || dir == "" {
		dir = "/usr/share/zoneinfo"
	}
	return dir + "/" + name
}

// posixZone is the zone glibc's __tzset_parse_tz makes of a rule string.
// Go's footer evaluation (its internal tzset) agrees with glibc from 1970
// on, but two glibc behaviours need help:
//
//   - tzset.c compute_change counts every year <= 1970 from 1970-01-01, so
//     the rule's changes always lie in 1970 and no DST boundary is ever
//     crossed before 1970: every earlier instant has the type in force at
//     the epoch (standard time for a northern rule, DST for a southern one).
//     The TZif therefore holds one transition at 0, and the type before it
//     is the epoch's.
//   - A rule Go cannot evaluate keeps what glibc parsed of it before giving
//     up: the standard name (or "") and offset (or 0), not Go's "UTC".
func posixZone(rule string) *time.Location {
	stdName, stdOff := glibcStdTime(rule)
	probe, err := time.LoadLocationFromTZData(rule, posixTZif(rule, stdName, stdOff, false, false))
	if err != nil {
		return nil
	}
	name, off := time.Unix(0, 0).In(probe).Zone()
	dst := time.Unix(0, 0).In(probe).IsDST()
	loc, err := time.LoadLocationFromTZData(rule, posixTZif(rule, name, off, dst, true))
	if err != nil {
		return nil
	}
	return loc
}

// glibcStdTime is tzset.c parse_tzname and parse_offset for the standard
// time at the head of rule, with glibc's defaults on failure ("" and 0).
func glibcStdTime(rule string) (string, int) {
	n := 0
	for n < len(rule) && isAlpha(rule[n]) {
		n++
	}
	name, rest := rule[:n], rule[n:]
	if n < 3 {
		if rule == "" || rule[0] != '<' {
			return "", 0
		}
		n = 1
		for n < len(rule) && (isAlnum(rule[n]) || rule[n] == '+' || rule[n] == '-') {
			n++
		}
		if n >= len(rule) || rule[n] != '>' || n-1 < 3 {
			return "", 0
		}
		name, rest = rule[1:n], rule[n+1:]
	}
	sign := -1
	if rest != "" && (rest[0] == '+' || rest[0] == '-') {
		if rest[0] == '-' {
			sign = 1
		}
		rest = rest[1:]
	}
	var hms [3]uint16 // sscanf "%hu:%hu:%hu"
	for i := range hms {
		if i > 0 {
			if rest == "" || rest[0] != ':' {
				break
			}
			rest = rest[1:]
		}
		d := 0
		for d < len(rest) && isDigit(rest[d]) {
			hms[i] = hms[i]*10 + uint16(rest[d]-'0')
			d++
		}
		if d == 0 {
			if i == 0 {
				return name, 0
			}
			break
		}
		rest = rest[d:]
	}
	// compute_offset clamps each field
	off := int(min(hms[2], 59)) + int(min(hms[1], 59))*60 + int(min(hms[0], 24))*3600
	return name, sign * off
}

func isAlpha(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }

// posixTZif wraps a POSIX TZ rule in a TZif v2 file with one type (name,
// utoff, isdst) and, if epoch is set, one transition to it at 0. Go applies
// the footer rule to every instant from the last transition on (all of
// them without one), uses the type before it, and falls back to the type
// when it cannot evaluate the rule.
func posixTZif(rule, name string, utoff int, isdst, epoch bool) []byte {
	var timecnt uint32
	if epoch {
		timecnt = 1
	}
	desig := name + "\x00"
	header := func(b []byte) []byte {
		b = append(b, "TZif2"...)
		b = append(b, make([]byte, 15)...)
		for _, n := range []uint32{0, 0, 0, timecnt, 1, uint32(len(desig))} { // isut, isstd, leap, time, type, char
			b = binary.BigEndian.AppendUint32(b, n)
		}
		return b
	}
	block := func(b []byte, timeSize int) []byte {
		if epoch {
			b = append(b, make([]byte, timeSize)...) // transition time 0
			b = append(b, 0)                         // to type 0
		}
		b = binary.BigEndian.AppendUint32(b, uint32(int32(utoff)))
		var d byte
		if isdst {
			d = 1
		}
		b = append(b, d, 0) // isdst, desigidx 0
		return append(b, desig...)
	}
	var b []byte
	b = block(header(b), 4)
	b = block(header(b), 8)
	b = append(b, '\n')
	b = append(b, rule...)
	return append(b, '\n')
}

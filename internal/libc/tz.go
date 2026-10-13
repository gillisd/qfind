package libc

import (
	"encoding/binary"
	"os"
	"strings"
	"time"
)

// LoadTZ resolves $TZ like glibc tzset(): unset → /etc/localtime (Go's
// time.Local); empty → UTC; ":name" or "name" → zoneinfo file; otherwise a
// POSIX rule string such as "EST5" or "JST-9", which Go's time package
// silently turns into UTC.
func LoadTZ(lookup func(string) (string, bool)) *time.Location {
	tz, ok := lookup("TZ")
	if !ok {
		return time.Local
	}
	name := strings.TrimPrefix(tz, ":")
	if name == "" {
		return time.UTC
	}
	if data, err := os.ReadFile(tzfilePath(name, lookup)); err == nil {
		if loc, err := time.LoadLocationFromTZData(name, data); err == nil {
			return loc
		}
	}
	if loc, err := time.LoadLocationFromTZData(name, posixTZif(name)); err == nil {
		return loc
	}
	return time.UTC
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

// posixTZif wraps a POSIX TZ rule in a TZif v2 file with no transitions and
// one placeholder type. Go applies the footer rule (its internal tzset) to
// every instant after the last transition, which here is all of them.
func posixTZif(rule string) []byte {
	header := func(b []byte, version byte, typecnt, charcnt uint32) []byte {
		b = append(b, "TZif"...)
		b = append(b, version)
		b = append(b, make([]byte, 15)...)
		for _, n := range []uint32{0, 0, 0, 0, typecnt, charcnt} { // isut, isstd, leap, time, type, char
			b = binary.BigEndian.AppendUint32(b, n)
		}
		return b
	}
	block := func(b []byte) []byte {
		b = append(b, 0, 0, 0, 0, 0, 0) // ttinfo: utoff 0, isdst 0, desigidx 0
		return append(b, "UTC\x00"...)
	}
	var b []byte
	b = block(header(b, '2', 1, 4))
	b = block(header(b, '2', 1, 4))
	b = append(b, '\n')
	b = append(b, rule...)
	return append(b, '\n')
}

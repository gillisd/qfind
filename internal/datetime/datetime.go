// Package datetime is gnulib parse-datetime.y (findutils 4.11.0): the date
// strings of find -newerXY with Y = t.
package datetime

//go:generate go tool goyacc -l -o parse.go parse.y

import (
	"strings"
	"time"

	"qfind/internal/libc"
)

// Timespec is struct timespec.
type Timespec struct{ Sec, Nsec int64 }

type timespec struct{ sec, nsec int64 }

// textint is an integer and the number of digits in its text.
type textint struct {
	negative bool
	value    int64
	digits   int64
}

// relTime is relative_time; ns is a C int.
type relTime struct {
	year, month, day, hour, minutes, seconds int64
	ns                                       int64
}

// parserControl is parser_control without the debug fields.
type parserControl struct {
	dayOrdinal     int64
	dayNumber      int64
	localIsDST     int64
	timeZone       int64 // seconds east of UT (C int)
	meridian       int
	year           textint
	month, day     int64
	hour, minutes  int64
	seconds        timespec
	rel            relTime
	timespecSeen   bool
	relsSeen       bool
	datesSeen      int
	daysSeen       int
	jZonesSeen     int
	localZonesSeen int
	dstsSeen       int
	timesSeen      int
	zonesSeen      int
	yearSeen       bool
	notDecimal     bool
	// localTimeZoneTable holds at most two entries; an empty name is the
	// NULL name that ends it.
	localTimeZoneTable [3]table
	aborted            bool
}

// abort is YYABORT: parsing may go on, but Parse then rejects the input.
func (pc *parserControl) abort() { pc.aborted = true }

// setHHMMSS is set_hhmmss.
func (pc *parserControl) setHHMMSS(h, m, s, ns int64) {
	pc.hour, pc.minutes, pc.seconds = h, m, timespec{s, ns}
}

// dayUnit is the relunit actions' ckd_mul(&$$.day, n, unit).
func (pc *parserControl) dayUnit(n, unit int64) relTime {
	d, ovf := ckdMul(n, unit)
	if ovf {
		pc.abort()
	}
	return relTime{day: d}
}

// digitsToDateTime is digits_to_date_time: YYYYMMDD, HHMM, HH, or a year.
func (pc *parserControl) digitsToDateTime(t textint) {
	if pc.datesSeen > 0 && pc.year.digits == 0 && !pc.relsSeen && (pc.timesSeen > 0 || 2 < t.digits) {
		pc.yearSeen, pc.year = true, t
		return
	}
	if 4 < t.digits {
		pc.datesSeen++
		pc.day, pc.month = t.value%100, (t.value/100)%100
		pc.year.value, pc.year.digits = t.value/10000, t.digits-4
		return
	}
	pc.timesSeen++
	if t.digits <= 2 {
		pc.hour, pc.minutes = t.value, 0
	} else {
		pc.hour, pc.minutes = t.value/100, t.value%100
	}
	pc.seconds, pc.meridian = timespec{}, mer24
}

// applyRelativeTime is apply_relative_time (factor 1 or -1). Every field
// is updated even after an overflow, as the C code's '|' chain does.
func (pc *parserControl) applyRelativeTime(rel relTime, factor int64) bool {
	op := ckdAdd
	if factor < 0 {
		op = ckdSub
	}
	var ovf, o bool
	if pc.rel.ns, o = op(pc.rel.ns, rel.ns); o || !fitsInt(pc.rel.ns) { // ns is a C int
		ovf = true
		pc.rel.ns = int64(int32(pc.rel.ns))
	}
	for _, f := range []struct {
		dst *int64
		v   int64
	}{
		{&pc.rel.seconds, rel.seconds}, {&pc.rel.minutes, rel.minutes}, {&pc.rel.hour, rel.hour},
		{&pc.rel.day, rel.day}, {&pc.rel.month, rel.month}, {&pc.rel.year, rel.year},
	} {
		if *f.dst, o = op(*f.dst, f.v); o {
			ovf = true
		}
	}
	if ovf {
		return false
	}
	pc.relsSeen = true
	return true
}

// timeZoneHHMM is time_zone_hhmm: HH or HHMM, or HH and MM with mm >= 0,
// within ±24:00.
func (pc *parserControl) timeZoneHHMM(s textint, mm int64) bool {
	if s.digits <= 2 && mm < 0 {
		s.value *= 100
	}
	var n int64
	if mm < 0 {
		n = (s.value/100)*60 + s.value%100
	} else {
		var o1, o2 bool
		n, o1 = ckdMul(s.value, 60)
		if s.negative {
			n, o2 = ckdSub(n, mm)
		} else {
			n, o2 = ckdAdd(n, mm)
		}
		if o1 || o2 {
			return false
		}
	}
	if n < -24*60 || n > 24*60 {
		return false
	}
	pc.timeZone = n * 60
	return true
}

// toHour is to_hour.
func toHour(hours int64, meridian int) int64 {
	switch meridian {
	case merAM:
		switch {
		case 0 < hours && hours < 12:
			return hours
		case hours == 12:
			return 0
		}
		return -1
	case merPM:
		switch {
		case 0 < hours && hours < 12:
			return hours + 12
		case hours == 12:
			return 12
		}
		return -1
	}
	if 0 <= hours && hours < 24 {
		return hours
	}
	return -1
}

// toTmYear is to_tm_year: two-digit years 00-68 are 20xx, 69-99 19xx. A
// negative year becomes -1900 - year, as the C code computes it; false
// when the result does not fit an int.
func toTmYear(t textint) (int64, bool) {
	year := t.value
	if 0 <= year && t.digits == 2 {
		if year < 69 {
			year += 2000
		} else {
			year += 1900
		}
	}
	var y int64
	var ovf bool
	if year < 0 {
		y, ovf = ckdSub(-1900, year)
	} else {
		y, ovf = ckdSub(year, 1900)
	}
	return y, !ovf && fitsInt(y)
}

// populateLocalTimeZoneTable is populate_local_time_zone_table.
func (pc *parserControl) populateLocalTimeZoneTable(tm *tmT) {
	i := 0
	if pc.localTimeZoneTable[0].name != "" {
		i = 1
	}
	pc.localTimeZoneTable[i] = table{tm.zone, tLOCAL_ZONE, tm.isdst}
	pc.localTimeZoneTable[i+1].name = ""
}

// Parse is gnulib parse_datetime(result, s, now) with the default zone
// tzalloc(getenv("TZ")), passed in as loc.
func Parse(s string, now Timespec, loc *time.Location) (Timespec, bool) {
	if i := strings.IndexByte(s, 0); i >= 0 { // a C string ends at NUL
		s = s[:i]
	}
	p := strings.TrimLeft(s, " \t\n\v\f\r")
	tz := newZone(loc)
	if strings.HasPrefix(p, `TZ="`) { // TZ="zone" prefix, with \\ and \" escapes
		if name, rest, ok := quotedTZ(p[4:]); ok {
			tz = newZone(libc.TZFromString(name))
			p = strings.TrimLeft(rest, " \t\n\v\f\r")
		}
	}
	tmp, ok := localtime(now.Sec, tz)
	if !ok {
		return Timespec{}, false
	}
	if p == "" {
		p = "0" // the empty string means "0", not an error during a DST change
	}
	pc := &parserControl{
		year:    textint{value: tmp.year + 1900},
		month:   tmp.mon + 1,
		day:     tmp.mday,
		hour:    tmp.hour,
		minutes: tmp.min,
		seconds: timespec{tmp.sec, now.Nsec},
	}
	pc.meridian = mer24
	pc.populateLocalTimeZoneTable(&tmp)
	// Probe the next three quarters for a tm_isdst different from now's.
	for quarter := int64(1); quarter <= 3; quarter++ {
		t, ovf := ckdAdd(now.Sec, quarter*90*24*60*60)
		if ovf {
			break
		}
		probe, ok := localtime(t, tz)
		if !ok || (pc.localTimeZoneTable[0].name != "" && probe.isdst == pc.localTimeZoneTable[0].value) {
			continue
		}
		pc.populateLocalTimeZoneTable(&probe)
		if pc.localTimeZoneTable[1].name != "" {
			if pc.localTimeZoneTable[0].name == pc.localTimeZoneTable[1].name {
				// One abbreviation for both standard and daylight time.
				pc.localTimeZoneTable[0].value = -1
				pc.localTimeZoneTable[1].name = ""
			}
			break
		}
	}
	if yyParse(&lexer{pc: pc, in: p}) != 0 || pc.aborted {
		return Timespec{}, false
	}
	if pc.timespecSeen {
		return Timespec{pc.seconds.sec, pc.seconds.nsec}, true
	}
	return pc.resolve(tz, tmp.isdst)
}

// resolve is the rest of parse_datetime_body: it turns the parsed fields
// into a time in tz. isdst is the tm_isdst of now.
func (pc *parserControl) resolve(tz zone, isdst int64) (Timespec, bool) {
	if 1 < max(pc.timesSeen, pc.datesSeen, pc.daysSeen, pc.dstsSeen, pc.jZonesSeen+pc.localZonesSeen+pc.zonesSeen) {
		return Timespec{}, false
	}
	var tm tmT
	var ok bool
	tm.isdst = isdst
	if tm.year, ok = toTmYear(pc.year); !ok {
		return Timespec{}, false
	}
	tm.mon, tm.mday = pc.month-1, pc.day
	if !fitsInt(tm.mon, tm.mday) {
		return Timespec{}, false
	}
	if pc.timesSeen > 0 || (pc.relsSeen && pc.datesSeen == 0 && pc.daysSeen == 0) {
		if tm.hour = toHour(pc.hour, pc.meridian); tm.hour < 0 {
			return Timespec{}, false
		}
		// tm_min and tm_sec are C ints: the assignment truncates.
		tm.min, tm.sec = int64(int32(pc.minutes)), int64(int32(pc.seconds.sec))
	} else {
		tm.hour, tm.min, tm.sec, pc.seconds.nsec = 0, 0, 0, 0
	}
	if pc.datesSeen > 0 || pc.daysSeen > 0 || pc.timesSeen > 0 {
		tm.isdst = -1 // let mktime decide for an absolute time
	}
	if pc.localZonesSeen > 0 {
		tm.isdst = pc.localIsDST
	}
	tm0 := tm
	start, ok := mktimeZ(&tm, tz)
	if !ok || !tm0.sameFields(&tm) {
		if pc.zonesSeen == 0 {
			return Timespec{}, false
		}
		// Retry in a zone at the parsed offset: tzalloc("XXX" + offset).
		tm = tm0
		if start, ok = mktimeZ(&tm, xxxZone(pc.timeZone)); !ok || !tm0.sameFields(&tm) {
			return Timespec{}, false
		}
	}
	if pc.daysSeen > 0 && pc.datesSeen == 0 { // "next Tuesday"
		ord := pc.dayOrdinal
		if 0 < ord && tm.wday != pc.dayNumber {
			ord--
		}
		dayincr, o1 := ckdMul(ord, 7)
		if o1 {
			return Timespec{}, false
		}
		dayincr, o2 := ckdAdd((pc.dayNumber-tm.wday+7)%7, dayincr)
		mday, o3 := ckdAdd(dayincr, tm.mday)
		if o2 || o3 || !fitsInt(mday) {
			return Timespec{}, false
		}
		tm.mday, tm.isdst = mday, -1
		if start, ok = mktimeZ(&tm, tz); !ok {
			return Timespec{}, false
		}
	}
	if pc.rel.year != 0 || pc.rel.month != 0 || pc.rel.day != 0 { // relative date
		year, o1 := ckdAdd(tm.year, pc.rel.year)
		month, o2 := ckdAdd(tm.mon, pc.rel.month)
		day, o3 := ckdAdd(tm.mday, pc.rel.day)
		if o1 || o2 || o3 || !fitsInt(year, month, day) {
			return Timespec{}, false
		}
		tm.year, tm.mon, tm.mday = year, month, day
		tm.hour, tm.min, tm.sec, tm.isdst = tm0.hour, tm0.min, tm0.sec, tm0.isdst
		if start, ok = mktimeZ(&tm, tz); !ok {
			return Timespec{}, false
		}
	}
	if pc.zonesSeen > 0 { // the parsed zone replaces the local offset
		delta, o1 := ckdSub(pc.timeZone, tm.gmtoff)
		t1, o2 := ckdSub(start, delta)
		if o1 || o2 {
			return Timespec{}, false
		}
		start = t1
	}
	// Relative hours, minutes and seconds, ignoring leap seconds.
	sumNs := pc.seconds.nsec + pc.rel.ns
	normNs := (sumNs%1e9 + 1e9) % 1e9
	d4 := (sumNs - normNs) / 1e9
	d1, o1 := ckdMul(pc.rel.hour, 60*60)
	t1, o2 := ckdAdd(start, d1)
	d2, o3 := ckdMul(pc.rel.minutes, 60)
	t2, o4 := ckdAdd(t1, d2)
	t3, o5 := ckdAdd(t2, pc.rel.seconds)
	t4, o6 := ckdAdd(t3, d4)
	if o1 || o2 || o3 || o4 || o5 || o6 {
		return Timespec{}, false
	}
	return Timespec{t4, normNs}, true
}

// quotedTZ reads `zone"` after TZ=", undoing \\ and \" escapes.
func quotedTZ(s string) (name, rest string, ok bool) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			if i+1 >= len(s) || (s[i+1] != '\\' && s[i+1] != '"') {
				return "", "", false
			}
			i++
			b.WriteByte(s[i])
		case '"':
			return b.String(), s[i+1:], true
		default:
			b.WriteByte(c)
		}
	}
	return "", "", false
}

// xxxZone is tzalloc("XXX" + time_zone_str(tz)): glibc reads the POSIX
// offset "+HH[:MM[:SS]]" (west of UT, so the sign flips) and clamps each
// part (tzset.c compute_offset: hours to 24, minutes and seconds to 59).
func xxxZone(tz int64) zone {
	sign := int64(-1) // "+HH" is west of UT
	if tz < 0 {
		sign = 1
	}
	h, rem := min(abs64(tz/3600), 24), abs64(tz%3600)
	off := h*3600 + min(rem/60, 59)*60 + min(rem%60, 59)
	return zone{time.FixedZone("XXX", int(sign*off)), true}
}

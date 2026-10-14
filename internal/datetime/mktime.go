package datetime

import (
	"math"
	"sync"
	"time"

	"qfind/internal/libc"
)

// tmT is struct tm (years since 1900, months 0-11), plus tm_gmtoff and
// tm_zone. Fields are int64 so that C int overflow can be checked.
type tmT struct {
	year, mon, mday, hour, min, sec int64
	wday, yday, isdst, gmtoff       int64
	zone                            string
}

// zone is a timezone_t: the location, and whether glibc would evaluate it
// as a POSIX rule string with no tzfile behind it.
type zone struct {
	loc   *time.Location
	posix bool
}

func newZone(loc *time.Location) zone { return zone{loc, libc.IsPOSIXRule(loc)} }

// localtime is localtime_rz(z, &t): false, like glibc's EOVERFLOW, when
// the year does not fit tm_year (or, for a POSIX rule zone, when the UTC
// year does not).
func localtime(t int64, z zone) (tmT, bool) {
	// Far outside every int tm_year; also keeps Go's arithmetic in range.
	if t < -(1<<56) || t > 1<<56 {
		return tmT{}, false
	}
	u := time.Unix(t, 0)
	if z.posix && !fitsInt(int64(u.UTC().Year())-1900) {
		return tmT{}, false
	}
	lt := u.In(z.loc)
	abbr, off := lt.Zone()
	isdst := int64(0)
	if lt.IsDST() {
		isdst = 1
	}
	tm := tmT{year: int64(lt.Year()) - 1900, mon: int64(lt.Month()) - 1, mday: int64(lt.Day()),
		hour: int64(lt.Hour()), min: int64(lt.Minute()), sec: int64(lt.Second()),
		wday: int64(lt.Weekday()), yday: int64(lt.YearDay()) - 1, isdst: isdst,
		gmtoff: int64(off), zone: abbr}
	return tm, fitsInt(tm.year)
}

// sameFields is mktime_ok's comparison of the requested and the
// normalised fields (tm_isdst is not compared).
func (a *tmT) sameFields(b *tmT) bool {
	return a.sec == b.sec && a.min == b.min && a.hour == b.hour &&
		a.mday == b.mday && a.mon == b.mon && a.year == b.year
}

// localtimeOffset is glibc mktime.c's static localtime_offset: the guess
// at the UTC offset that each mktime call starts from and leaves for the
// next. It persists across calls and zones, so an ambiguous local time
// resolves as it does in one GNU find process.
var localtimeOffset struct {
	sync.Mutex
	off int64
}

// mktimeZ is gnulib mktime_z(z, tm) over glibc mktime: on success tm
// becomes the normalised time (wday, yday, isdst, gmtoff and zone filled
// in); on failure tm is left alone and ok is false.
func mktimeZ(tm *tmT, z zone) (int64, bool) {
	localtimeOffset.Lock()
	defer localtimeOffset.Unlock()
	return mktimeInternal(tm, z, &localtimeOffset.off)
}

// monYday is __mon_yday: the days before each month, normal and leap.
var monYday = [2][13]int64{
	{0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334, 365},
	{0, 31, 60, 91, 121, 152, 182, 213, 244, 274, 305, 335, 366},
}

// leapyear reports whether year + 1900 is a leap year.
func leapyear(year int64) int {
	if year&3 == 0 && (year%100 != 0 || (year/100)&3 == (-(1900/100))&3) {
		return 1
	}
	return 0
}

// isdstDiffer is isdst_differ: one zero and the other positive.
func isdstDiffer(a, b int64) bool {
	return (a == 0) != (b == 0) && 0 <= a && 0 <= b
}

// ydhmsDiff is ydhms_diff: (year1-yday1 hour1:min1:sec1) minus
// (year0-yday0 hour0:min0:sec0) in seconds, ignoring leap seconds.
func ydhmsDiff(year1, yday1, hour1, min1, sec1, year0, yday0, hour0, min0, sec0 int64) int64 {
	a4 := year1>>2 + 1900>>2 - b2i(year1&3 == 0)
	b4 := year0>>2 + 1900>>2 - b2i(year0&3 == 0)
	a100 := (a4+b2i(a4 < 0))/25 - b2i(a4 < 0)
	b100 := (b4+b2i(b4 < 0))/25 - b2i(b4 < 0)
	a400 := a100 >> 2
	b400 := b100 >> 2
	intervening := (a4 - b4) - (a100 - b100) + (a400 - b400)
	years := year1 - year0
	days := 365*years + yday1 - yday0 + intervening
	hours := 24*days + hour1 - hour0
	minutes := 60*hours + min1 - min0
	return 60*minutes + sec1 - sec0
}

// tmDiff is tm_diff: the requested time minus *tp.
func tmDiff(year, yday, hour, mi, sec int64, tp *tmT) int64 {
	return ydhmsDiff(year, yday, hour, mi, sec, tp.year, tp.yday, tp.hour, tp.min, tp.sec)
}

// longIntAvg is long_int_avg: the average rounded toward +Inf.
func longIntAvg(a, b int64) int64 {
	return a>>1 + b>>1 + (a|b)&1
}

// rangedConvert is ranged_convert: localtime of *t or, when that
// overflows, of the in-range value nearest it (binary search toward 0).
func rangedConvert(t *int64, z zone) (tmT, bool) {
	if tm, ok := localtime(*t, z); ok {
		return tm, true
	}
	bad, good := *t, int64(0)
	var goodTm tmT
	found := false
	for {
		mid := longIntAvg(good, bad)
		if mid == good || mid == bad {
			break
		}
		if tm, ok := localtime(mid, z); ok {
			good, goodTm, found = mid, tm, true
		} else {
			bad = mid
		}
	}
	if !found {
		return tmT{}, false
	}
	*t = good
	return goodTm, true
}

// mktimeInternal is glibc mktime.c __mktime_internal (local = true).
func mktimeInternal(tp *tmT, z zone, offset *int64) (int64, bool) {
	remainingProbes := 6 // _LIBC; gnulib doubles it

	sec, mi, hour := tp.sec, tp.min, tp.hour
	mday, mon, yearRequested, isdst := tp.mday, tp.mon, tp.year, tp.isdst

	dst2 := false // the previous probe was DST

	monRemainder := mon % 12
	negativeMonRemainder := b2i(monRemainder < 0)
	monYears := mon/12 - negativeMonRemainder
	year := yearRequested + monYears

	yday := monYday[leapyear(year)][monRemainder+12*negativeMonRemainder] - 1 + mday

	off := *offset
	secRequested := sec
	sec = max(0, min(sec, 59)) // LEAP_SECONDS_POSSIBLE

	negativeOffsetGuess := int64(int32(-off)) // ckd_sub into an int
	t0 := ydhmsDiff(year, yday, hour, mi, sec, 1970-1900, 0, 0, 0, negativeOffsetGuess)
	t, t1, t2 := t0, t0, t0

	var tm tmT
	for {
		var ok bool
		if tm, ok = rangedConvert(&t, z); !ok {
			return -1, false
		}
		dt := tmDiff(year, yday, hour, mi, sec, &tm)
		if dt == 0 {
			break
		}
		if t == t1 && t != t2 && (tm.isdst < 0 || oscillating(isdst, dst2, tm.isdst)) {
			// A spring-forward gap: return a time dt away from the
			// requested one.
			goto offsetFound
		}
		remainingProbes--
		if remainingProbes == 0 {
			return -1, false
		}
		t1, t2, t, dst2 = t2, t, t+dt, tm.isdst != 0
	}

	if isdstDiffer(isdst, tm.isdst) {
		// Probe for a neighbouring time with the requested isdst and use
		// its UTC offset. glibc 2.44 still bounds the search by half the
		// longest DST period of tzdata2003a (America/Jujuy, 536454000 s),
		// not by the year that findutils' gnulib mktime.c uses; date -d
		// "1950-06-01 IST DST" in Asia/Kolkata (DST until 1945) shows it.
		const stride = 601200
		const durationMax = 536454000
		const deltaBound = durationMax/2 + stride
		for delta := int64(stride); delta < deltaBound; delta += stride {
			for direction := int64(-1); direction <= 1; direction += 2 {
				ot, ovf := ckdAdd(t, delta*direction)
				if ovf {
					continue
				}
				otm, ok := rangedConvert(&ot, z)
				if !ok {
					return -1, false
				}
				if isdstDiffer(isdst, otm.isdst) {
					continue
				}
				gt := ot + tmDiff(year, yday, hour, mi, sec, &otm)
				if gtm, ok := localtime(gt, z); ok {
					t, tm = gt, gtm
					goto offsetFound
				}
			}
		}
		// No probe has the requested isdst. glibc 2.44, unlike findutils'
		// gnulib copy, then assumes a one-hour DST shift: TZ=UTC
		// mktime({2020-01-15 12:00, isdst 1}) is 11:00 UTC.
		gt, ovf := ckdAdd(t, 3600*(b2i(isdst == 0)-b2i(tm.isdst == 0)))
		if ovf {
			return -1, false
		}
		gtm, ok := localtime(gt, z)
		if !ok {
			return -1, false
		}
		t, tm = gt, gtm
	}

offsetFound:
	*offset = t - t0 - negativeOffsetGuess

	if secRequested != tm.sec {
		// Honour the requested tm_sec rather than the normalised one.
		adj := b2i(sec == 0 && tm.sec == 60) - sec + secRequested
		nt, ovf := ckdAdd(t, adj)
		if ovf {
			return -1, false
		}
		t = nt
		var ok bool
		if tm, ok = localtime(t, z); !ok {
			return -1, false
		}
	}
	*tp = tm
	return t, true
}

// oscillating is the spring-forward test of __mktime_internal's probe
// loop: with no isdst requested, dst2 <= (tm_isdst != 0); otherwise the
// probe's isdst differs from the requested one.
func oscillating(isdst int64, dst2 bool, tmIsdst int64) bool {
	if isdst < 0 {
		return !dst2 || tmIsdst != 0
	}
	return (isdst != 0) != (tmIsdst != 0)
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// fitsInt reports whether every value fits a C int.
func fitsInt(vs ...int64) bool {
	for _, v := range vs {
		if v != int64(int32(v)) {
			return false
		}
	}
	return true
}

func ckdAdd(a, b int64) (int64, bool) {
	s := a + b
	return s, (b > 0 && s < a) || (b < 0 && s > a)
}

func ckdSub(a, b int64) (int64, bool) {
	d := a - b
	return d, (b > 0 && d > a) || (b < 0 && d < a)
}

func ckdMul(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, false
	}
	p := a * b
	return p, p/b != a || (a == -1 && b == math.MinInt64) || (b == -1 && a == math.MinInt64)
}

// ckdAddInt is ckd_add on a C int (pc->time_zone).
func ckdAddInt(a, b int64) (int64, bool) {
	s, ovf := ckdAdd(a, b)
	return s, ovf || !fitsInt(s)
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

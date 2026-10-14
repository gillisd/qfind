package libc

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	wdayNames = [...]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	monNames  = [...]string{"January", "February", "March", "April", "May", "June", "July",
		"August", "September", "October", "November", "December"}
)

// Strftime is glibc strftime(3) under LC_TIME=C for the formats find uses.
func Strftime(format string, t time.Time) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			b.WriteByte(format[i])
			continue
		}
		j := i + 1
		if j < len(format) && (format[j] == 'E' || format[j] == 'O') {
			j++
		}
		if j >= len(format) {
			b.WriteString(format[i:]) // bad_format at the end
			break
		}
		if s, ok := convert(format[j], t); ok && modifierOK(format[j-1], format[j]) {
			b.WriteString(s)
		} else {
			b.WriteString(format[i : j+1])
		}
		i = j
	}
	return b.String()
}

// modifierOK reports whether glibc accepts the E or O modifier m (the byte
// before c, which is '%' when there is none) on conversion c; a rejected
// pair is bad_format and copied literally, as "%Ea" is.
func modifierOK(m, c byte) bool {
	switch {
	case m == '%':
		return true
	case strings.IndexByte("aADF", c) >= 0:
		return false
	case m == 'E':
		return strings.IndexByte("bBhdegGHIjklmMSUVwW", c) < 0
	}
	return strings.IndexByte("cxXY", c) < 0
}

// floorDiv and floorMod round toward negative infinity: glibc 2.44 prints
// year -1902 as %C "-20" and %y "98".
func floorDiv(a, b int) int {
	q := a / b
	if a%b < 0 {
		q--
	}
	return q
}

func floorMod(a, b int) int {
	return a - b*floorDiv(a, b)
}

func pad2(v int) string { return fmt.Sprintf("%02d", v) }

// isoWeekDays is strftime_l.c iso_week_days (Monday weeks, Thursday rule).
func isoWeekDays(yday, wday int) int {
	const bigEnoughMultipleOf7 = (366/7 + 2) * 7
	return yday - (yday-wday+4+bigEnoughMultipleOf7)%7 + 4 - 1
}

func isLeap(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

func convert(c byte, t time.Time) (string, bool) {
	year, yday, wday := t.Year(), t.YearDay()-1, int(t.Weekday())
	hour12 := t.Hour() % 12
	if hour12 == 0 {
		hour12 = 12
	}
	switch c {
	case 'a':
		return wdayNames[wday][:3], true
	case 'A':
		return wdayNames[wday], true
	case 'b', 'h':
		return monNames[t.Month()-1][:3], true
	case 'B':
		return monNames[t.Month()-1], true
	case 'c':
		return Strftime("%a %b %e %H:%M:%S %Y", t), true
	case 'C':
		return strconv.Itoa(floorDiv(year, 100)), true
	case 'd':
		return pad2(t.Day()), true
	case 'D', 'x':
		return Strftime("%m/%d/%y", t), true
	case 'e':
		return fmt.Sprintf("%2d", t.Day()), true
	case 'F':
		return Strftime("%Y-%m-%d", t), true
	case 'g', 'G', 'V':
		adjust := 0
		days := isoWeekDays(yday, wday)
		if days < 0 {
			adjust = -1
			n := 365
			if isLeap(year - 1) {
				n = 366
			}
			days = isoWeekDays(yday+n, wday)
		} else {
			n := 365
			if isLeap(year) {
				n = 366
			}
			if d := isoWeekDays(yday-n, wday); d >= 0 {
				adjust, days = 1, d
			}
		}
		switch c {
		case 'g':
			return pad2(floorMod(year+adjust, 100)), true
		case 'G':
			return strconv.Itoa(year + adjust), true
		}
		return pad2(days/7 + 1), true
	case 'H':
		return pad2(t.Hour()), true
	case 'I':
		return pad2(hour12), true
	case 'j':
		return fmt.Sprintf("%03d", yday+1), true
	case 'k':
		return fmt.Sprintf("%2d", t.Hour()), true
	case 'l':
		return fmt.Sprintf("%2d", hour12), true
	case 'm':
		return pad2(int(t.Month())), true
	case 'M':
		return pad2(t.Minute()), true
	case 'n':
		return "\n", true
	case 'p':
		if t.Hour() < 12 {
			return "AM", true
		}
		return "PM", true
	case 'P':
		if t.Hour() < 12 {
			return "am", true
		}
		return "pm", true
	case 'r':
		return Strftime("%I:%M:%S %p", t), true
	case 'R':
		return Strftime("%H:%M", t), true
	case 's':
		return strconv.FormatInt(t.Unix(), 10), true
	case 'S':
		return pad2(t.Second()), true
	case 't':
		return "\t", true
	case 'T', 'X':
		return Strftime("%H:%M:%S", t), true
	case 'u':
		if wday == 0 {
			return "7", true
		}
		return strconv.Itoa(wday), true
	case 'U':
		return pad2((yday - wday + 7) / 7), true
	case 'w':
		return strconv.Itoa(wday), true
	case 'W':
		return pad2((yday - (wday-1+7)%7 + 7) / 7), true
	case 'y':
		return pad2(floorMod(year, 100)), true
	case 'Y':
		return strconv.Itoa(year), true
	case 'z':
		_, off := t.Zone()
		sign := '+'
		if off < 0 {
			sign, off = '-', -off
		}
		return fmt.Sprintf("%c%02d%02d", sign, off/3600, off/60%60), true
	case 'Z':
		name, _ := t.Zone()
		return name, true
	case '%':
		return "%", true
	}
	return "", false
}

package find

import (
	"math"
	"strconv"
	"strings"
)

// cspec is the "%[-+ #][width][.prec]" part of a segment, parsed as C
// does (a leading 0 in the width is the 0 flag).
type cspec struct {
	minus, plus, space, hash, zero bool
	width                          int
	prec                           int  // -1 when absent
	overflow                       bool // width or precision past INT_MAX: vfprintf fails with EOVERFLOW
}

// parseCspec parses the flags, width and precision text after '%', as
// print.c get_format_flags_length delimited it.
func parseCspec(s string) cspec {
	c := cspec{prec: -1}
	i := 0
	for ; i < len(s) && strings.IndexByte("-+ #", s[i]) >= 0; i++ {
		switch s[i] {
		case '-':
			c.minus = true
		case '+':
			c.plus = true
		case ' ':
			c.space = true
		case '#':
			c.hash = true
		}
	}
	if i < len(s) && s[i] == '0' {
		c.zero = true
	}
	j := i
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	var ok1, ok2 bool
	c.width, ok1 = cint(s[i:j])
	ok2 = true
	if j < len(s) && s[j] == '.' {
		c.prec, ok2 = cint(s[j+1:])
	}
	c.overflow = !ok1 || !ok2
	return c
}

// cint reads a run of decimal digits ("" is 0) as glibc's read_int does:
// false when the value does not fit an int.
func cint(s string) (int, bool) {
	n := 0
	for i := 0; i < len(s); i++ {
		n = 10*n + int(s[i]-'0')
		if n > math.MaxInt32 {
			return 0, false
		}
	}
	return n, true
}

// pad applies the width: left-justified with '-', zero-filled after the
// sign when the 0 flag counts for this conversion, else right-justified.
func pad(c cspec, body string, zeroOK bool) string {
	if len(body) >= c.width {
		return body
	}
	n := c.width - len(body)
	switch {
	case c.minus:
		return body + strings.Repeat(" ", n)
	case c.zero && zeroOK:
		sign := ""
		if body != "" && (body[0] == '-' || body[0] == '+' || body[0] == ' ') {
			sign, body = body[:1], body[1:]
		}
		return sign + strings.Repeat("0", n) + body
	}
	return strings.Repeat(" ", n) + body
}

// cfmtS is %s: precision truncates bytes, only '-' and width matter
// (glibc pads %s with spaces even under the 0 flag).
func cfmtS(c cspec, s string) string {
	if c.prec >= 0 && len(s) > c.prec {
		s = s[:c.prec]
	}
	return pad(c, s, false)
}

// cfmtD is %d with C semantics (precision = minimum digits, disables 0).
func cfmtD(c cspec, v int64) string {
	mag := uint64(v)
	if v < 0 {
		mag = -mag
	}
	digits := strconv.FormatUint(mag, 10)
	if c.prec >= 0 {
		if c.prec == 0 && v == 0 {
			digits = ""
		}
		digits = leftZeros(digits, c.prec)
	}
	sign := ""
	switch {
	case v < 0:
		sign = "-"
	case c.plus:
		sign = "+"
	case c.space:
		sign = " "
	}
	return pad(c, sign+digits, c.prec < 0)
}

// cfmtO is %o: '#' forces a leading 0; '+' and ' ' do nothing.
func cfmtO(c cspec, v uint64) string {
	digits := strconv.FormatUint(v, 8)
	if c.prec >= 0 {
		if c.prec == 0 && v == 0 {
			digits = ""
		}
		digits = leftZeros(digits, c.prec)
	}
	if c.hash && !strings.HasPrefix(digits, "0") {
		digits = "0" + digits
	}
	return pad(c, digits, c.prec < 0)
}

func leftZeros(digits string, n int) string {
	if len(digits) >= n {
		return digits
	}
	return strings.Repeat("0", n-len(digits)) + digits
}

// cfmtG is %g: precision P (default 6, 0 means 1), %e style when the
// exponent X < -4 or X >= P, trailing zeros removed unless '#'.
func cfmtG(c cspec, v float64) string {
	sign := ""
	switch {
	case math.Signbit(v):
		sign, v = "-", -v
	case c.plus:
		sign = "+"
	case c.space:
		sign = " "
	}
	if math.IsInf(v, 0) {
		return pad(c, sign+"inf", false)
	}
	p := c.prec
	if p < 0 {
		p = 6
	} else if p == 0 {
		p = 1
	}
	x := 0
	if v != 0 {
		// The exponent after rounding to P significant digits.
		r := strconv.FormatFloat(v, 'e', p-1, 64)
		x, _ = strconv.Atoi(r[strings.IndexByte(r, 'e')+1:])
	}
	var body string
	if x < -4 || x >= p {
		mant, exp, _ := strings.Cut(strconv.FormatFloat(v, 'e', p-1, 64), "e")
		if !c.hash {
			mant = trimZeros(mant)
		} else if !strings.Contains(mant, ".") {
			mant += "."
		}
		e, _ := strconv.Atoi(exp)
		es := "+"
		if e < 0 {
			es, e = "-", -e
		}
		body = mant + "e" + es + leftZeros(strconv.Itoa(e), 2)
	} else {
		body = strconv.FormatFloat(v, 'f', p-1-x, 64)
		if !c.hash {
			body = trimZeros(body)
		} else if !strings.Contains(body, ".") {
			body += "."
		}
	}
	return pad(c, sign+body, true)
}

func trimZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

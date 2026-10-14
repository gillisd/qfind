package libc

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Strtod is glibc strtod(3) in the C locale. It returns the value of the
// longest prefix of s (after white space) that is a number, the length of
// that prefix (0 when there is none), and whether errno would be ERANGE:
// overflow to ±HUGE_VAL, underflow to zero, or an inexact subnormal result.
//
// Accepted: [+-] then "inf", "infinity" or "nan[(chars)]" in any case; a
// hexadecimal "0x" mantissa with optional "p[+-]digits" (Go's ParseFloat
// requires the exponent, so it is supplied); or decimal digits with an
// optional '.' and an "e[+-]digits" exponent consumed only when digits
// follow it.
func Strtod(s string) (v float64, n int, erange bool) {
	i := 0
	for i < len(s) && IsSpace(s[i]) {
		i++
	}
	start := i
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	sign := func(f float64) float64 {
		if neg {
			return -f
		}
		return f
	}
	lower := strings.ToLower(s[i:min(len(s), i+8)])
	switch {
	case strings.HasPrefix(lower, "infinity"):
		return sign(math.Inf(1)), i + 8, false
	case strings.HasPrefix(lower, "inf"):
		return sign(math.Inf(1)), i + 3, false
	case strings.HasPrefix(lower, "nan"):
		j := i + 3
		if j < len(s) && s[j] == '(' {
			k := j + 1
			for k < len(s) && (isAlnum(s[k]) || s[k] == '_') {
				k++
			}
			if k < len(s) && s[k] == ')' {
				j = k + 1
			}
		}
		// glibc keeps the sign of "-nan" (strtod_l.c: negative ? -retval).
		return sign(math.NaN()), j, false
	}
	if i+1 < len(s) && s[i] == '0' && (s[i+1] == 'x' || s[i+1] == 'X') {
		j := i + 2
		digits := 0
		for j < len(s) && isHex(s[j]) {
			j++
			digits++
		}
		if j < len(s) && s[j] == '.' {
			j++
			for j < len(s) && isHex(s[j]) {
				j++
				digits++
			}
		}
		if digits > 0 {
			mant := s[i+2 : j]
			exp := "p0"
			if j < len(s) && (s[j] == 'p' || s[j] == 'P') {
				k := j + 1
				if k < len(s) && (s[k] == '+' || s[k] == '-') {
					k++
				}
				if k < len(s) && isDigit(s[k]) {
					for k < len(s) && isDigit(s[k]) {
						k++
					}
					exp, j = "p"+s[j+1:k], k
				}
			}
			lit := "0x" + mant + exp
			f, err := strconv.ParseFloat(lit, 64)
			return sign(f), j, rangeError(f, err, strings.Trim(mant, "0.") != "", lit)
		}
		// "0x" without hex digits: the number is the "0".
	}
	j, digits := i, 0
	for j < len(s) && isDigit(s[j]) {
		j++
		digits++
	}
	if j < len(s) && s[j] == '.' {
		j++
		for j < len(s) && isDigit(s[j]) {
			j++
			digits++
		}
	}
	if digits == 0 {
		return 0, 0, false
	}
	nonzero := strings.ContainsAny(s[i:j], "123456789")
	if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
		k := j + 1
		if k < len(s) && (s[k] == '+' || s[k] == '-') {
			k++
		}
		if k < len(s) && isDigit(s[k]) {
			for k < len(s) && isDigit(s[k]) {
				k++
			}
			j = k
		}
	}
	f, err := strconv.ParseFloat(s[start:j], 64)
	return f, j, rangeError(f, err, nonzero, s[i:j])
}

// rangeError reports glibc's ERANGE (strtod_l.c round_and_return):
// overflow, underflow to zero, or a subnormal result that is inexact.
// lit is the unsigned literal, used only to test a subnormal's exactness.
func rangeError(f float64, err error, nonzero bool, lit string) bool {
	if err != nil {
		return true // strconv.ErrRange: ±Inf
	}
	a := math.Abs(f)
	switch {
	case a == 0:
		return nonzero
	case a >= 0x1p-1022:
		return false
	}
	r, ok := new(big.Rat).SetString(lit)
	if !ok {
		return true
	}
	return r.Cmp(new(big.Rat).SetFloat64(a)) != 0
}

// Xstrtod is gnulib xstrtod(s, NULL, &v, strtod): the whole string must be
// one number, and ERANGE fails only on overflow (|v| >= 1); gradual and
// flush-to-zero underflow are not errors.
func Xstrtod(s string) (float64, bool) {
	v, n, erange := Strtod(s)
	if n == 0 || n != len(s) {
		return v, false
	}
	if v != 0 && erange && (v >= 1 || v <= -1) {
		return v, false
	}
	return v, true
}

// IsSpace is isspace(3) in the C locale.
func IsSpace(c byte) bool { return c == ' ' || (c >= '\t' && c <= '\r') }

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isHex(c byte) bool   { return isDigit(c) || (c|0x20 >= 'a' && c|0x20 <= 'f') }
func isAlnum(c byte) bool { return isDigit(c) || (c|0x20 >= 'a' && c|0x20 <= 'z') }

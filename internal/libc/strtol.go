package libc

import "math/bits"

// Xstrtoumax is gnulib xstrtoumax(s, NULL, 10, &v, "") == LONGINT_OK over
// glibc strtoumax: optional white space and '+', then decimal digits to
// the end of s. A '-' (even after white space), no digits, a trailing
// character (valid_suffixes is "") and overflow all fail.
func Xstrtoumax(s string) (uint64, bool) {
	i := 0
	for i < len(s) && IsSpace(s[i]) {
		i++
	}
	if i < len(s) && s[i] == '-' {
		return 0, false
	}
	if i < len(s) && s[i] == '+' {
		i++
	}
	start := i
	var v uint64
	overflow := false
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		hi, lo := bits.Mul64(v, 10)
		sum, carry := bits.Add64(lo, uint64(s[i]-'0'), 0)
		overflow = overflow || hi != 0 || carry != 0
		v = sum
	}
	if i == start || i != len(s) || overflow {
		return 0, false
	}
	return v, true
}

// IsSpace is isspace(3) in the C locale.
func IsSpace(c byte) bool { return c == ' ' || (c >= '\t' && c <= '\r') }

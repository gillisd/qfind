package regex

import "unicode"

// glibc's UTF-8 mbrtowc (iconv/gconv_simple.c) is the original, wider
// UTF-8: sequences of up to six bytes encode values up to 0x7fffffff.
// It rejects overlong forms and surrogates; everything else that is well
// formed is one character, so "\xf8\x88\x80\x80\x80" (U+200000) is a
// single character to '.' and to brackets.

// decodeChar is mbrtowc on s[i:]: the character and its length, or n == 0
// for an invalid or incomplete sequence (which glibc's regex then treats
// as a single byte).
func decodeChar(s []byte, i int) (r rune, n int) {
	c := s[i]
	var min rune
	switch {
	case c < 0x80:
		return rune(c), 1
	case c < 0xc2:
		return 0, 0
	case c < 0xe0:
		n, r, min = 2, rune(c&0x1f), 0x80
	case c < 0xf0:
		n, r, min = 3, rune(c&0x0f), 0x800
	case c < 0xf8:
		n, r, min = 4, rune(c&0x07), 0x10000
	case c < 0xfc:
		n, r, min = 5, rune(c&0x03), 0x200000
	case c < 0xfe:
		n, r, min = 6, rune(c&0x01), 0x4000000
	default:
		return 0, 0
	}
	if i+n > len(s) {
		return 0, 0
	}
	for k := 1; k < n; k++ {
		d := s[i+k]
		if d&0xc0 != 0x80 {
			return 0, 0
		}
		r = r<<6 | rune(d&0x3f)
	}
	if r < min || (0xd800 <= r && r <= 0xdfff) {
		return 0, 0
	}
	return r, n
}

// charBytes is wcrtomb's length for a character decodeChar returns.
func charBytes(r rune) int {
	switch {
	case r < 0x80:
		return 1
	case r < 0x800:
		return 2
	case r < 0x10000:
		return 3
	case r < 0x200000:
		return 4
	case r < 0x4000000:
		return 5
	}
	return 6
}

// appendChar is wcrtomb: r in glibc's UTF-8.
func appendChar(b []byte, r rune) []byte {
	n := charBytes(r)
	if n == 1 {
		return append(b, byte(r))
	}
	lead := [...]byte{0, 0, 0xc0, 0xe0, 0xf0, 0xf8, 0xfc}[n]
	b = append(b, lead|byte(r>>(6*(n-1))))
	for k := n - 2; k >= 0; k-- {
		b = append(b, 0x80|byte(r>>(6*k))&0x3f)
	}
	return b
}

// towupper is glibc's towupper in C.UTF-8 (it agrees with Go's tables
// for every Unicode code point; values past U+10FFFF map to themselves).
func towupper(r rune) rune {
	if r > unicode.MaxRune {
		return r
	}
	return unicode.ToUpper(r)
}

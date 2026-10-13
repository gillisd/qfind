package libc

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Quote is quotearg_n_style(n, locale_quoting_style, s), find's
// err_quoting_style. Quotes are '…' unless LC_CTYPE is UTF-8, which gives
// U+2018/U+2019 (gnulib gettext_quote). Control characters get C escapes,
// '\\' and the closing quote are backslash-escaped, and non-printable bytes
// (every byte >= 0x80 in a single-byte locale) become \ooo.
func (l Locale) Quote(s string) string {
	left, right := "'", "'"
	if l.UTF8 {
		left, right = "\u2018", "\u2019"
	}
	var b strings.Builder
	b.WriteString(left)
	for i := 0; i < len(s); {
		c := s[i]
		if esc := cEscape(c); esc != "" {
			b.WriteString(esc)
			i++
			continue
		}
		if strings.HasPrefix(s[i:], right) {
			b.WriteByte('\\')
			b.WriteString(right)
			i += len(right)
			continue
		}
		if c >= 0x20 && c < 0x7f {
			b.WriteByte(c)
			i++
			continue
		}
		if l.UTF8 && c >= 0x80 {
			// A literal U+FFFD is a valid character; only (RuneError, 1)
			// marks an invalid byte.
			if r, n := utf8.DecodeRuneInString(s[i:]); (r != utf8.RuneError || n > 1) && isPrint(r) {
				b.WriteString(s[i : i+n])
				i += n
				continue
			}
		}
		b.WriteByte('\\')
		b.WriteString(octal3(c))
		i++
	}
	b.WriteString(right)
	return b.String()
}

func cEscape(c byte) string {
	switch c {
	case '\a':
		return `\a`
	case '\b':
		return `\b`
	case '\f':
		return `\f`
	case '\n':
		return `\n`
	case '\r':
		return `\r`
	case '\t':
		return `\t`
	case '\v':
		return `\v`
	case '\\':
		return `\\`
	}
	return ""
}

func octal3(c byte) string {
	o := strconv.FormatUint(uint64(c), 8)
	return strings.Repeat("0", 3-len(o)) + o
}

// Qmark is print_quoted() for a terminal destination: literal quoting
// followed by qmark_chars(), which turns every non-printable character
// (byte in a single-byte locale, character in UTF-8) into '?'.
func (l Locale) Qmark(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		if c >= 0x20 && c < 0x7f {
			b.WriteByte(c)
			i++
			continue
		}
		if l.UTF8 && c >= 0x80 {
			if truncatedUTF8(s[i:]) {
				// mbrtowc's (size_t) -2: qmark_chars replaces the
				// incomplete character at the end with one '?'.
				b.WriteByte('?')
				break
			}
			r, n := utf8.DecodeRuneInString(s[i:])
			if r != utf8.RuneError || n > 1 {
				if isPrint(r) {
					b.WriteString(s[i : i+n])
				} else {
					b.WriteByte('?')
				}
				i += n
				continue
			}
		}
		b.WriteByte('?')
		i++
	}
	return b.String()
}

// truncatedUTF8 reports whether glibc's mbrtowc returns (size_t) -2 for s:
// a lead byte followed only by continuation bytes, fewer than the lead byte
// announces. glibc still honours the old 5- and 6-byte leads here and
// checks no value ranges, so "\xf8\x80", "\xed\xa0" and "\xe0\x80" at the
// end are incomplete rather than invalid.
func truncatedUTF8(s string) bool {
	var need int
	switch c := s[0]; {
	case c >= 0xc2 && c < 0xe0:
		need = 2
	case c >= 0xe0 && c < 0xf0:
		need = 3
	case c >= 0xf0 && c < 0xf8:
		need = 4
	case c >= 0xf8 && c < 0xfc:
		need = 5
	case c >= 0xfc && c < 0xfe:
		need = 6
	default:
		return false
	}
	if len(s) >= need {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i]&0xc0 != 0x80 {
			return false
		}
	}
	return true
}

// isPrint is glibc's iswprint in a UTF-8 locale, which c32isprint and
// wcwidth() >= 0 both follow: every assigned character except the controls
// (Cc) and the line and paragraph separators (Zl, Zp). Unlike
// unicode.IsPrint, it accepts non-ASCII spaces (Zs), format characters (Cf)
// and private-use characters (Co).
func isPrint(r rune) bool {
	return unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S,
		unicode.Zs, unicode.Cf, unicode.Co)
}

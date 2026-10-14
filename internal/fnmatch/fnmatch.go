// Package fnmatch is glibc's fnmatch(3) for the flags GNU find passes:
// FNM_CASEFOLD or nothing (never FNM_PATHNAME, FNM_PERIOD, FNM_NOESCAPE,
// FNM_LEADING_DIR or FNM_EXTMATCH). GNU find links the system glibc's
// fnmatch, not gnulib's, so this ports glibc 2.44's posix/fnmatch_loop.c
// FCT along its _LIBC paths, with the C and C.UTF-8 locales' collation (no
// collating rules, code point order) and ctype tables. It differs from
// findutils' gl/lib/fnmatch_loop.c where glibc does: an unterminated
// bracket found while skipping a matched one is a literal '[', and a
// failed wide match falls back to bytes (see Match).
package fnmatch

import (
	"strings"
	"unsafe"
)

// Options are fnmatch's flags plus the process state glibc consults.
type Options struct {
	CaseFold       bool // FNM_CASEFOLD
	UTF8           bool // MB_CUR_MAX > 1: match characters, not bytes
	PosixlyCorrect bool // POSIXLY_CORRECT set: '^' no longer negates
}

// charClassMaxLength is glibc's CHARCLASS_NAME_MAX.
const charClassMaxLength = 2048

// Match is fnmatch(pattern, name, flags) == 0, as glibc 2.44 has it
// (posix/fnmatch.c). In a UTF-8 locale it converts both strings with
// mbsrtowcs and matches characters; when a string is not valid UTF-8, or
// the wide matcher reports FNM_NOMATCH, it falls back to matching bytes.
// So a name matches when either matcher accepts it. All-ASCII strings
// match the same either way, so they take the byte matcher alone.
func Match(pattern, name string, o Options) bool {
	pattern, name = cString(pattern), cString(name)
	m := matcher{fold: o.CaseFold, posix: o.PosixlyCorrect, utf8Locale: o.UTF8}
	if o.UTF8 && !(isASCII(pattern) && isASCII(name)) {
		wp, ok1 := mbsrtowcs(pattern)
		wn, ok2 := mbsrtowcs(name)
		w := m
		w.wide = true
		if ok1 && ok2 && fct(&w, wp, 0, wn, 0, nil) {
			return true
		}
	}
	return fct(&m, bytesOf(pattern), 0, bytesOf(name), 0, nil)
}

// cString is s as C sees it, up to the first NUL.
func cString(s string) string {
	if i := strings.IndexByte(s, 0); i >= 0 {
		return s[:i]
	}
	return s
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// minValue is the smallest value an n-byte UTF-8 sequence may encode.
var minValue = [7]rune{0, 0, 0x80, 0x800, 0x10000, 0x200000, 0x4000000}

// mbsrtowcs is mbsrtowcs(3) in glibc's UTF-8 locales. Unlike Go's
// unicode/utf8 it takes the original six-byte form, so values above
// U+10FFFF up to 0x7fffffff convert; overlong forms and surrogates do not.
func mbsrtowcs(s string) ([]rune, bool) {
	out := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		c := s[i]
		var n int
		var v rune
		switch {
		case c < 0x80:
			out = append(out, rune(c))
			i++
			continue
		case c&0xe0 == 0xc0:
			n, v = 2, rune(c&0x1f)
		case c&0xf0 == 0xe0:
			n, v = 3, rune(c&0x0f)
		case c&0xf8 == 0xf0:
			n, v = 4, rune(c&0x07)
		case c&0xfc == 0xf8:
			n, v = 5, rune(c&0x03)
		case c&0xfe == 0xfc:
			n, v = 6, rune(c&0x01)
		default:
			return nil, false
		}
		if i+n > len(s) {
			return nil, false
		}
		for j := 1; j < n; j++ {
			b := s[i+j]
			if b&0xc0 != 0x80 {
				return nil, false
			}
			v = v<<6 | rune(b&0x3f)
		}
		if v < minValue[n] || (v >= 0xd800 && v <= 0xdfff) {
			return nil, false
		}
		out = append(out, v)
		i += n
	}
	return out, true
}

// bytesOf views s as a read-only byte slice without copying it.
func bytesOf(s string) []byte { return unsafe.Slice(unsafe.StringData(s), len(s)) }

// matcher is the flags and locale state FCT reads: internal_fnmatch when
// wide is false, internal_fnwmatch when it is true.
type matcher struct {
	fold       bool // FNM_CASEFOLD
	posix      bool // posixly_correct > 0
	wide       bool
	utf8Locale bool
}

// FOLD.
func (m *matcher) FOLD(c rune) rune {
	switch {
	case !m.fold:
		return c
	case m.wide:
		return towlower(c)
	}
	return tolower(c)
}

// unknownSeq is __collseq_table_lookup's "no information" ~0.
const unknownSeq = ^uint32(0)

// collseq is the collation sequence value of c: _NL_COLLATE_COLLSEQMB's
// identity table in the byte matcher, __collseq_table_lookup in the wide
// one, where C.UTF-8's table holds only U+0000..U+00FF (in code point
// order) and every other character is unknown.
func (m *matcher) collseq(c rune) uint32 {
	if m.wide && c > 0xff {
		return unknownSeq
	}
	return uint32(c)
}

// ends is struct STRUCT: where an inner FCT stopped at a '*'.
type ends struct {
	p, n int
	set  bool
}

type char interface{ ~byte | ~rune }

// at is *(p + i) with the terminating NUL past the end.
func at[T char](s []T, i int) rune {
	if i < len(s) {
		return rune(s[i])
	}
	return 0
}

// fct is fnmatch_loop.c FCT: match s[ni:] against p[pi:]. With e non-nil it
// succeeds at the next '*', recording where.
func fct[T char](m *matcher, p []T, pi int, s []T, ni int, e *ends) bool {
	for {
		c := at(p, pi)
		pi++
		if c == 0 {
			break
		}
		c = m.FOLD(c)
		switch c {
		case '?':
			if ni == len(s) {
				return false
			}
		case '\\':
			c = at(p, pi)
			pi++
			if c == 0 {
				return false // trailing \ loses
			}
			c = m.FOLD(c)
			if ni == len(s) || m.FOLD(rune(s[ni])) != c {
				return false
			}
		case '*':
			if e != nil {
				e.p, e.n, e.set = pi-1, ni, true
				return true
			}
			c = at(p, pi)
			pi++
			for ; c == '?' || c == '*'; c, pi = at(p, pi), pi+1 {
				if c == '?' {
					if ni == len(s) {
						return false
					}
					ni++
				}
			}
			if c == 0 {
				return true
			}
			bracket := c == '['
			if c == '\\' {
				c = at(p, pi)
			}
			c = m.FOLD(c)
			pi--
			var end ends
			found := false
			for ; ni < len(s); ni++ {
				if (bracket || m.FOLD(rune(s[ni])) == c) && fct(m, p, pi, s, ni, &end) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
			if !end.set {
				return true
			}
			pi, ni = end.p, end.n
			continue
		case '[':
			if ni == len(s) {
				return false
			}
			next, ok := bracketExpr(m, p, pi, rune(s[ni]))
			switch ok {
			case bracketNoMatch:
				return false
			case bracketLiteral: // [ unterminated: a normal character
				if '[' != m.FOLD(rune(s[ni])) {
					return false
				}
			default:
				pi = next
			}
		default:
			if ni == len(s) || c != m.FOLD(rune(s[ni])) {
				return false
			}
		}
		ni++
	}
	return ni == len(s)
}

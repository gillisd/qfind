package fnmatch

import "sort"

// class is a wctype_t: zero for a name wctype rejects.
type class uint8

const (
	classNone class = iota
	classAlnum
	classAlpha
	classBlank
	classCntrl
	classDigit
	classGraph
	classLower
	classPrint
	classPunct
	classSpace
	classUpper
	classXdigit
	classCombining // i18n's extra class: UTF-8 locales only
)

var classNames = map[string]class{
	"alnum": classAlnum, "alpha": classAlpha, "blank": classBlank,
	"cntrl": classCntrl, "digit": classDigit, "graph": classGraph,
	"lower": classLower, "print": classPrint, "punct": classPunct,
	"space": classSpace, "upper": classUpper, "xdigit": classXdigit,
	"combining": classCombining,
}

// wctype is wctype(3) for the locales qfind distinguishes: the C locale
// knows the twelve POSIX classes, glibc's UTF-8 locales add "combining"
// (their "combining_level3" cannot be named, see scanClass).
func wctype(name string, utf8Locale bool) class {
	c := classNames[name]
	if c == classCombining && !utf8Locale {
		return classNone
	}
	return c
}

// isctype is iswctype(c, cl) in the wide matcher. The byte matcher's
// _ISCTYPE only ever sees bytes, and glibc classifies none above 0x7f in
// the C locale or a UTF-8 one, so it passes wide=false.
func isctype(cl class, c rune, wide bool) bool {
	if !wide && c >= 0x80 {
		return false
	}
	switch cl {
	case classAlnum:
		return isDigit(c) || inTable(alphaTable, c)
	case classAlpha:
		return inTable(alphaTable, c)
	case classBlank:
		return inTable(blankTable, c)
	case classCntrl:
		return inTable(cntrlTable, c)
	case classDigit:
		return isDigit(c)
	case classGraph:
		return inTable(graphTable, c)
	case classLower:
		return inTable(lowerTable, c)
	case classPrint: // glibc: graph plus the blanks other than tab
		return inTable(graphTable, c) || (c != '\t' && inTable(blankTable, c))
	case classPunct: // glibc: graph and not alnum
		return inTable(graphTable, c) && !isDigit(c) && !inTable(alphaTable, c)
	case classSpace:
		return inTable(spaceTable, c)
	case classUpper:
		return inTable(upperTable, c)
	case classXdigit:
		return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
	case classCombining:
		return inTable(combiningTable, c)
	}
	return false
}

func isDigit(c rune) bool { return c >= '0' && c <= '9' }

// inTable reports whether c lies in one of t's inclusive [lo, hi] pairs.
func inTable(t []rune, c rune) bool {
	n := len(t) / 2
	i := sort.Search(n, func(i int) bool { return t[2*i+1] >= c })
	return i < n && t[2*i] <= c
}

// towlower is glibc's towlower in its UTF-8 locales.
func towlower(c rune) rune {
	i := sort.Search(len(lowerRuns), func(i int) bool { return lowerRuns[i][1] >= c })
	if i == len(lowerRuns) {
		return c
	}
	r := lowerRuns[i]
	if c < r[0] || (c-r[0])%r[3] != 0 {
		return c
	}
	return c + r[2]
}

// tolower is tolower(3) on a byte: ASCII only, in the C locale and in
// glibc's UTF-8 locales alike.
func tolower(c rune) rune {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

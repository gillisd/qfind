package regex

import "slices"

// classNames are the names wctype accepts.
var classNames = []string{"alnum", "alpha", "blank", "cntrl", "digit", "graph", "lower", "print", "punct", "space", "upper", "xdigit"}

// isClass is the C library is<class> on a byte, in the C locale and in
// C.UTF-8 alike: only ASCII bytes are characters of any class.
func isClass(name string, b int) bool {
	if b < 0 || b >= 0x80 {
		return false
	}
	lower, upper, digit := 'a' <= b && b <= 'z', 'A' <= b && b <= 'Z', '0' <= b && b <= '9'
	graph := 0x21 <= b && b <= 0x7e
	switch name {
	case "alnum":
		return lower || upper || digit
	case "alpha":
		return lower || upper
	case "blank":
		return b == ' ' || b == '\t'
	case "cntrl":
		return b < 0x20 || b == 0x7f
	case "digit":
		return digit
	case "graph":
		return graph
	case "lower":
		return lower
	case "print":
		return graph || b == ' '
	case "punct":
		return graph && !lower && !upper && !digit
	case "space":
		return b == ' ' || '\t' <= b && b <= '\r'
	case "upper":
		return upper
	case "xdigit":
		return digit || 'a' <= b|0x20 && b|0x20 <= 'f'
	}
	return false
}

// isWideClass is iswctype in C.UTF-8; ctype_tables.go is generated from
// glibc's own tables. WEOF (-1) belongs to no class.
func isWideClass(name string, wc rune) bool {
	if wc < 0x80 {
		return isClass(name, int(wc))
	}
	_, found := slices.BinarySearchFunc(wideClasses[name], wc, func(r [2]rune, wc rune) int {
		switch {
		case r[1] < wc:
			return -1
		case r[0] > wc:
			return 1
		}
		return 0
	})
	return found
}

// isWideWordChar is IS_WIDE_WORD_CHAR: iswalnum or '_'.
func isWideWordChar(wc rune) bool { return wc == '_' || isWideClass("alnum", wc) }

// bitset is bitset_t: one bit per byte value.
type bitset [4]uint64

func (b *bitset) set(c int)           { b[c>>6] |= 1 << (c & 63) }
func (b *bitset) clear(c int)         { b[c>>6] &^= 1 << (c & 63) }
func (b *bitset) contains(c int) bool { return b[c>>6]>>(c&63)&1 != 0 }
func (b *bitset) not() {
	for i := range b {
		b[i] = ^b[i]
	}
}
func (b *bitset) merge(o *bitset) {
	for i := range b {
		b[i] |= o[i]
	}
}
func (b *bitset) mask(o *bitset) {
	for i := range b {
		b[i] &= o[i]
	}
}
func (b *bitset) empty() bool { return b[0]|b[1]|b[2]|b[3] == 0 }

// utf8SBMap is utf8_sb_map: the single-byte characters of UTF-8 (ASCII).
var utf8SBMap = bitset{^uint64(0), ^uint64(0), 0, 0}

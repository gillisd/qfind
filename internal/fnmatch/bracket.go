package fnmatch

// bracketResult is how FCT's `case L_('['):` ends.
type bracketResult uint8

const (
	bracketNoMatch bracketResult = iota // return FNM_NOMATCH
	bracketMatch                        // break: the expression matched
	bracketLiteral                      // unterminated: match '[' itself
)

// bracketExpr is FCT's `case L_('['):` against the string character sc,
// with pi just past the '['. It returns the pattern index after the
// closing ']'. The [= =] and [. .] branches take the nrules == 0 paths.
func bracketExpr[T char](m *matcher, p []T, pi int, sc rune) (int, bracketResult) {
	not := at(p, pi) == '!' || (!m.posix && at(p, pi) == '^')
	if not {
		pi++
	}
	fn := m.FOLD(sc)
	c := at(p, pi)
	pi++
	for {
		var cold rune
		normal := false // goto normal_bracket
		member := false // in FCT's last else block, which handles ranges
		switch {
		case c == '\\':
			if at(p, pi) == 0 {
				return 0, bracketNoMatch
			}
			c = m.FOLD(at(p, pi))
			pi++
			normal = true
		case c == '[' && at(p, pi) == ':':
			startp := pi
			var name []rune
			valid := true
			for {
				if len(name) == charClassMaxLength {
					return 0, bracketNoMatch // name too long: ill-formed
				}
				pi++
				c = at(p, pi)
				if c == ':' && at(p, pi+1) == ']' {
					pi += 2
					break
				}
				if c < 'a' || c >= 'z' { // not a class name: a normal '['
					pi, c, valid = startp, '[', false
					break
				}
				name = append(name, c)
			}
			if !valid {
				normal = true
				break
			}
			wt := wctype(string(name), m.utf8Locale)
			if wt == classNone {
				return 0, bracketNoMatch
			}
			if isctype(wt, sc, m.wide) {
				return skipMatched(p, pi, not)
			}
			c = at(p, pi)
			pi++
		case c == '[' && at(p, pi) == '=':
			startp := pi
			pi++
			c = at(p, pi)
			if c == 0 {
				pi, c, normal = startp, '[', true
				break
			}
			str := c
			pi++
			c = at(p, pi)
			if c != '=' || at(p, pi+1) != ']' {
				pi, c, normal = startp, '[', true
				break
			}
			pi += 2
			if sc == str {
				return skipMatched(p, pi, not)
			}
			c = at(p, pi)
			pi++
		case c == 0:
			return 0, bracketLiteral
		default:
			if c == '[' && at(p, pi) == '.' {
				startp := pi
				c1 := 0
				for {
					pi++
					c = at(p, pi)
					if c == '.' && at(p, pi+1) == ']' {
						pi += 2
						break
					}
					if c == 0 {
						return 0, bracketNoMatch
					}
					c1++
				}
				isRange := at(p, pi) == '-' && at(p, pi+1) != 0
				if c1 != 1 {
					return 0, bracketNoMatch // no names in the collation data
				}
				if !isRange && sc == at(p, startp+1) {
					return skipMatched(p, pi, not)
				}
				cold = at(p, startp+1)
				c = at(p, pi)
				pi++
				member = true
			} else {
				c = m.FOLD(c)
				normal = true
			}
		}
		if normal { // normal_bracket:
			member = true
			isRange := at(p, pi) == '-' && at(p, pi+1) != 0 && at(p, pi+1) != ']'
			if !isRange && c == fn {
				return skipMatched(p, pi, not)
			}
			cold = c
			c = at(p, pi)
			pi++
		}
		if member && c == '-' && at(p, pi) != ']' {
			var matched, ok bool
			pi, matched, ok = rangeEnd(m, p, pi, fn, cold)
			if !ok {
				return 0, bracketNoMatch
			}
			if matched {
				return skipMatched(p, pi, not)
			}
			c = at(p, pi)
			pi++
		}
		if c == ']' {
			break
		}
	}
	if !not {
		return 0, bracketNoMatch
	}
	return pi, bracketMatch
}

// rangeEnd is FCT's range branch, `if (c == L_('-') && *p != L_(']'))`,
// with pi at the range's upper bound. It reports whether fn falls in
// [cold, cend] by collation sequence, and ok == false for FNM_NOMATCH.
func rangeEnd[T char](m *matcher, p []T, pi int, fn, cold rune) (next int, matched, ok bool) {
	cend := at(p, pi)
	pi++
	fcollseq := m.collseq(fn)
	if fcollseq == unknownSeq {
		// goto range_not_matched, before an escaped or [. .] upper bound
		// is read: only its first character has been consumed.
		return pi, false, true
	}
	lcollseq := m.collseq(cold)
	if cend == '[' && at(p, pi) == '.' {
		startp := pi
		c1 := 0
		for {
			pi++
			c := at(p, pi)
			if c == '.' && at(p, pi+1) == ']' {
				pi += 2
				break
			}
			if c == 0 {
				return 0, false, false
			}
			c1++
		}
		if c1 != 1 {
			return 0, false, false // no names in the collation data
		}
		cend = at(p, startp+1)
	} else {
		if cend == '\\' {
			cend = at(p, pi)
			pi++
		}
		if cend == 0 {
			return 0, false, false
		}
		cend = m.FOLD(cend)
	}
	if lcollseq == unknownSeq || lcollseq <= fcollseq {
		hcollseq := m.collseq(cend)
		if hcollseq == unknownSeq {
			// No information about the upper bound: the range matches
			// only its lower bound.
			return pi, lcollseq == fcollseq, true
		}
		if lcollseq <= hcollseq && fcollseq <= hcollseq {
			return pi, true, true
		}
	}
	return pi, false, true
}

// skipMatched is FCT's `matched:` label: skip the rest of the [...] that
// already matched, with its own (looser) idea of the nested brackets.
func skipMatched[T char](p []T, pi int, not bool) (int, bracketResult) {
	for {
		c := at(p, pi)
		pi++
		if c == ']' {
			break
		}
		switch {
		case c == 0:
			return 0, bracketLiteral // glibc 2.44: '[' unterminated, a normal character
		case c == '\\':
			if at(p, pi) == 0 {
				return 0, bracketNoMatch
			}
			pi++
		case c == '[' && at(p, pi) == ':':
			c1 := 0
			startp := pi
			for {
				pi++
				c = at(p, pi)
				c1++
				if c1 == charClassMaxLength {
					return 0, bracketNoMatch
				}
				if at(p, pi) == ':' && at(p, pi+1) == ']' {
					break
				}
				if c < 'a' || c >= 'z' {
					pi = startp - 2
					break
				}
			}
			pi += 2
		case c == '[' && at(p, pi) == '=':
			pi++
			if at(p, pi) == 0 {
				return 0, bracketNoMatch
			}
			pi++
			if at(p, pi) != '=' || at(p, pi+1) != ']' {
				return 0, bracketNoMatch
			}
			pi += 2
		case c == '[' && at(p, pi) == '.':
			for {
				pi++
				c = at(p, pi)
				if c == 0 {
					return 0, bracketNoMatch
				}
				if c == '.' && at(p, pi+1) == ']' {
					break
				}
			}
			pi += 2
		}
	}
	if not {
		return 0, bracketNoMatch
	}
	return pi, bracketMatch
}

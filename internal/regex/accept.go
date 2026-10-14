package regex

import "slices"

// byteAt is re_string_byte_at; glibc's buffer ends in a NUL.
func (p *reString) byteAt(i int) byte {
	if i >= len(p.mbs) {
		return 0
	}
	return p.mbs[i]
}

// checkNodeAcceptBytes is check_node_accept_bytes: how many bytes the
// multibyte-capable node accepts at strIdx (0 for none). glibc compares
// a range against the wide character as unsigned values, so WEOF (the
// middle of a character) is in no range.
func (m *matcher) checkNodeAcceptBytes(nodeIdx, strIdx int) int {
	d, in := m.d, m.in
	n := &d.nodes[nodeIdx]
	if n.typ == opUTF8Period {
		return utf8PeriodLen(in, strIdx)
	}
	charLen := in.charSizeAt(strIdx)
	if n.typ == opPeriod {
		if charLen <= 1 {
			return 0
		}
		b := in.byteAt(strIdx)
		if (d.syntax&DotNewline == 0 && b == '\n') || (d.syntax&DotNotNull != 0 && b == 0) {
			return 0
		}
		return charLen
	}
	if charLen <= 1 { // elem_len is 1 without collation rules
		return 0
	}
	if n.typ != complexBracket {
		return 0
	}
	cs := n.mbcset
	var wc rune
	if len(cs.rangeStarts) > 0 || len(cs.classes) > 0 || len(cs.mbchars) > 0 {
		wc = in.wcharAt(strIdx)
	}
	matched := slices.Contains(cs.mbchars, wc)
	for i := 0; !matched && i < len(cs.classes); i++ {
		matched = isWideClass(cs.classes[i], wc)
	}
	for i := 0; !matched && i < len(cs.rangeStarts); i++ {
		matched = cs.rangeStarts[i] <= uint32(wc) && uint32(wc) <= cs.rangeEnds[i]
	}
	if matched != cs.nonMatch {
		return charLen
	}
	return 0
}

// utf8PeriodLen is OP_UTF8_PERIOD in check_node_accept_bytes: a
// structurally well-formed sequence of 2 to 6 bytes (surrogates and values
// past U+10FFFF included).
func utf8PeriodLen(in *reString, i int) int {
	c := in.byteAt(i)
	if c < 0xc2 || i+2 > in.len {
		return 0
	}
	d := in.byteAt(i + 1)
	var n int
	switch {
	case c < 0xe0:
		if d < 0x80 || d > 0xbf {
			return 0
		}
		return 2
	case c < 0xf0:
		n = 3
		if c == 0xe0 && d < 0xa0 {
			return 0
		}
	case c < 0xf8:
		n = 4
		if c == 0xf0 && d < 0x90 {
			return 0
		}
	case c < 0xfc:
		n = 5
		if c == 0xf8 && d < 0x88 {
			return 0
		}
	case c < 0xfe:
		n = 6
		if c == 0xfc && d < 0x84 {
			return 0
		}
	default:
		return 0
	}
	if i+n > in.len {
		return 0
	}
	for k := 1; k < n; k++ {
		if b := in.byteAt(i + k); b < 0x80 || b > 0xbf {
			return 0
		}
	}
	return n
}

// checkNodeAccept is check_node_accept: does the node accept byte idx?
func (m *matcher) checkNodeAccept(nodeIdx, idx int) bool {
	d := m.d
	n := &d.nodes[nodeIdx]
	ch := m.in.byteAt(idx)
	switch n.typ {
	case character:
		if n.c != ch {
			return false
		}
	case simpleBracket:
		if !n.sbcset.contains(int(ch)) {
			return false
		}
	case opUTF8Period, opPeriod:
		if n.typ == opUTF8Period && ch >= 0x80 {
			return false
		}
		if (ch == '\n' && d.syntax&DotNewline == 0) || (ch == 0 && d.syntax&DotNotNull != 0) {
			return false
		}
	default:
		return false
	}
	return n.constraint == 0 || !notSatisfyNext(n.constraint, m.in.contextAt(idx))
}

// groupNodes is group_nodes_into_DFAstates: it partitions the bytes the
// state's nodes accept into destinations with the same node set.
func (m *matcher) groupNodes(s *state) ([]nodeSet, []bitset) {
	d := m.d
	var destsNode []nodeSet
	var destsCh []bitset
	for _, e := range s.nodes {
		n := &d.nodes[e]
		var accepts bitset
		switch n.typ {
		case character:
			accepts.set(int(n.c))
		case simpleBracket:
			accepts.merge(n.sbcset)
		case opPeriod:
			if d.mbCurMax > 1 {
				accepts.merge(d.sbChar)
			} else {
				accepts.not()
			}
			if d.syntax&DotNewline == 0 {
				accepts.clear('\n')
			}
			if d.syntax&DotNotNull != 0 {
				accepts.clear(0)
			}
		case opUTF8Period:
			accepts.merge(&utf8SBMap)
			if d.syntax&DotNewline == 0 {
				accepts.clear('\n')
			}
			if d.syntax&DotNotNull != 0 {
				accepts.clear(0)
			}
		default:
			continue
		}
		if c := n.constraint; c != 0 {
			if c&nextNewlineConstraint != 0 {
				nl := accepts.contains('\n')
				accepts = bitset{}
				if !nl {
					continue
				}
				accepts.set('\n')
			}
			if c&nextEndbufConstraint != 0 {
				continue
			}
			if c&nextWordConstraint != 0 {
				if n.typ == character && !n.wordChar {
					continue
				}
				var any uint64
				for j := range accepts {
					if d.mbCurMax > 1 {
						accepts[j] &= d.wordChar[j] | ^d.sbChar[j]
					} else {
						accepts[j] &= d.wordChar[j]
					}
					any |= accepts[j]
				}
				if any == 0 {
					continue
				}
			}
			if c&nextNotWordConstraint != 0 {
				if n.typ == character && n.wordChar {
					continue
				}
				var any uint64
				for j := range accepts {
					if d.mbCurMax > 1 {
						accepts[j] &^= d.wordChar[j] & d.sbChar[j]
					} else {
						accepts[j] &^= d.wordChar[j]
					}
					any |= accepts[j]
				}
				if any == 0 {
					continue
				}
			}
		}
		j := 0
		for ; j < len(destsNode); j++ {
			if n.typ == character && !destsCh[j].contains(int(n.c)) {
				continue
			}
			var intersec, remains bitset
			var hasIntersec, notSubset, notConsumed uint64
			for k := range accepts {
				intersec[k] = accepts[k] & destsCh[j][k]
				hasIntersec |= intersec[k]
			}
			if hasIntersec == 0 {
				continue
			}
			for k := range accepts {
				remains[k] = ^accepts[k] & destsCh[j][k]
				notSubset |= remains[k]
				accepts[k] &^= destsCh[j][k]
				notConsumed |= accepts[k]
			}
			if notSubset != 0 {
				destsCh = append(destsCh, remains)
				destsCh[j] = intersec
				destsNode = append(destsNode, destsNode[j].clone())
			}
			destsNode[j].insert(e)
			if notConsumed == 0 {
				break
			}
		}
		if j == len(destsNode) {
			destsCh = append(destsCh, accepts)
			destsNode = append(destsNode, newSet1(e))
		}
	}
	return destsNode, destsCh
}

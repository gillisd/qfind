package regex

import "slices"

// charset is re_charset_t, the COMPLEX_BRACKET part of a bracket in a
// multibyte locale. C and C.UTF-8 have no collation rules (nrules == 0),
// so there are never collating symbols or equivalence classes, and range
// ends are collation values, which for both locales are the code points.
type charset struct {
	mbchars     []rune
	rangeStarts []uint32
	rangeEnds   []uint32
	classes     []string
	nonMatch    bool
}

type elemType uint8

const (
	sbChar elemType = iota
	mbChar
	equivClass
	collSym
	charClass
)

// bracketElem is bracket_elem_t.
type bracketElem struct {
	typ  elemType
	ch   byte
	wch  rune
	name []byte
}

// bracketNameBufSize is BRACKET_NAME_BUF_SIZE.
const bracketNameBufSize = 32

// parseBracketExp is parse_bracket_exp.
func (p *parser) parseBracketExp(t *token, syn Syntax) *tree {
	d, in := p.dfa, p.in
	sbcset := &bitset{}
	mbcset := &charset{}
	nonMatch := false
	first := true

	tokenLen := p.peekTokenBracket(t, syn)
	if t.typ == endOfRE {
		p.err = regBadPat
		return nil
	}
	if t.typ == opNonMatchList {
		mbcset.nonMatch = true
		nonMatch = true
		if syn&HatListsNotNewline != 0 {
			sbcset.set('\n')
		}
		in.cur += tokenLen
		tokenLen = p.peekTokenBracket(t, syn)
		if t.typ == endOfRE {
			p.err = regBadPat
			return nil
		}
	}
	if t.typ == opCloseBracket { // a first ']' is ordinary
		t.typ = character
	}
	for {
		var start, end bracketElem
		var t2 token
		tokenLen2 := 0
		isRange := false
		start.typ = collSym
		if err := p.parseBracketElement(&start, t, tokenLen, syn, first); err != regNoError {
			p.err = err
			return nil
		}
		first = false
		tokenLen = p.peekTokenBracket(t, syn)
		if start.typ != charClass && start.typ != equivClass {
			if t.typ == endOfRE {
				p.err = regEBrack
				return nil
			}
			if t.typ == opCharsetRange {
				in.cur += tokenLen // skip '-'
				tokenLen2 = p.peekTokenBracket(&t2, syn)
				if t2.typ == endOfRE {
					p.err = regEBrack
					return nil
				}
				if t2.typ == opCloseBracket { // a last '-' is ordinary
					in.cur -= tokenLen
					t.typ = character
				} else {
					isRange = true
				}
			}
		}
		if isRange {
			end.typ = collSym
			if err := p.parseBracketElement(&end, &t2, tokenLen2, syn, true); err != regNoError {
				p.err = err
				return nil
			}
			tokenLen = p.peekTokenBracket(t, syn)
			if err := d.buildRangeExp(sbcset, mbcset, &start, &end, syn); err != regNoError {
				p.err = err
				return nil
			}
		} else {
			switch start.typ {
			case sbChar:
				sbcset.set(int(start.ch))
			case mbChar:
				mbcset.mbchars = append(mbcset.mbchars, start.wch)
			case equivClass, collSym:
				// build_equiv_class / build_collating_symbol, nrules == 0
				if len(start.name) != 1 {
					p.err = regECollate
					return nil
				}
				sbcset.set(int(start.name[0]))
			case charClass:
				if err := buildCharclass(sbcset, mbcset, string(start.name), syn); err != regNoError {
					p.err = err
					return nil
				}
			}
		}
		if t.typ == endOfRE {
			p.err = regEBrack
			return nil
		}
		if t.typ == opCloseBracket {
			break
		}
	}
	in.cur += tokenLen // skip ']'
	if nonMatch {
		sbcset.not()
	}
	if d.mbCurMax > 1 {
		sbcset.mask(d.sbChar)
	}
	if len(mbcset.mbchars) > 0 || len(mbcset.rangeStarts) > 0 ||
		(d.mbCurMax > 1 && (len(mbcset.classes) > 0 || mbcset.nonMatch)) {
		d.hasMBNode = true
		mbcTree := p.createTokenTree(nil, nil, &token{typ: complexBracket, mbcset: mbcset})
		if sbcset.empty() {
			return mbcTree
		}
		sbcTree := p.createTokenTree(nil, nil, &token{typ: simpleBracket, sbcset: sbcset})
		return p.createTree(sbcTree, mbcTree, opAlt)
	}
	return p.createTokenTree(nil, nil, &token{typ: simpleBracket, sbcset: sbcset})
}

// parseBracketElement is parse_bracket_element. A multibyte character is
// an MB_CHAR whatever token it starts.
func (p *parser) parseBracketElement(e *bracketElem, t *token, tokenLen int, syn Syntax, acceptHyphen bool) errcode {
	in := p.in
	if n := in.charSizeAt(in.cur); n > 1 {
		e.typ = mbChar
		e.wch = in.wcharAt(in.cur)
		in.cur += n
		return regNoError
	}
	in.cur += tokenLen
	if t.typ == opOpenCollElem || t.typ == opOpenCharClass || t.typ == opOpenEquivClass {
		return p.parseBracketSymbol(e, t)
	}
	if t.typ == opCharsetRange && !acceptHyphen {
		// a '-' that is not a range operator must end the list
		var t2 token
		p.peekTokenBracket(&t2, syn)
		if t2.typ != opCloseBracket {
			return regERange
		}
	}
	e.typ = sbChar
	e.ch = t.c
	return regNoError
}

// parseBracketSymbol is parse_bracket_symbol: [.name.], [=name=], [:name:].
func (p *parser) parseBracketSymbol(e *bracketElem, t *token) errcode {
	in := p.in
	delim := t.c
	if in.eoi() {
		return regEBrack
	}
	var name []byte
	for i := 0; ; i++ {
		if i >= bracketNameBufSize {
			return regEBrack
		}
		var ch byte
		if t.typ == opOpenCharClass {
			ch = in.fetchByteCase()
		} else {
			ch = in.fetchByte()
		}
		if in.eoi() {
			return regEBrack
		}
		if ch == delim && in.peekByte(0) == ']' {
			break
		}
		name = append(name, ch)
	}
	in.cur++
	e.name = name
	switch t.typ {
	case opOpenCollElem:
		e.typ = collSym
	case opOpenEquivClass:
		e.typ = equivClass
	case opOpenCharClass:
		e.typ = charClass
	}
	return regNoError
}

// collValue is lookup_collation_sequence_value with nrules == 0, where
// collseqmb is the identity: a single byte is its own value; a multibyte
// character has none.
func collValue(e *bracketElem) (uint32, bool) {
	switch e.typ {
	case sbChar:
		return uint32(e.ch), true
	case collSym:
		if len(e.name) == 1 {
			return uint32(e.name[0]), true
		}
	}
	return 0, false
}

// buildRangeExp is the _LIBC build_range_exp.
func (d *dfa) buildRangeExp(sbcset *bitset, mbcset *charset, start, end *bracketElem, syn Syntax) errcode {
	if start.typ == equivClass || start.typ == charClass || end.typ == equivClass || end.typ == charClass {
		return regERange
	}
	lo, ok1 := collValue(start)
	hi, ok2 := collValue(end)
	if !ok1 || !ok2 {
		return regECollate
	}
	if syn&NoEmptyRanges != 0 && lo > hi {
		return regERange
	}
	if d.mbCurMax > 1 {
		mbcset.rangeStarts = append(mbcset.rangeStarts, lo)
		mbcset.rangeEnds = append(mbcset.rangeEnds, hi)
	}
	for ch := uint32(0); ch < 256; ch++ {
		if lo <= ch && ch <= hi {
			sbcset.set(int(ch))
		}
	}
	return regNoError
}

// buildCharclass is build_charclass (trans is NULL for find): under
// RE_ICASE, upper and lower are alpha.
func buildCharclass(sbcset *bitset, mbcset *charset, name string, syn Syntax) errcode {
	if syn&ICase != 0 && (name == "upper" || name == "lower") {
		name = "alpha"
	}
	// glibc records __wctype(name) before validating the name; an
	// unknown name is an error either way.
	if !slices.Contains(classNames, name) {
		return regECtype
	}
	mbcset.classes = append(mbcset.classes, name)
	for i := 0; i < 256; i++ {
		if isClass(name, i) {
			sbcset.set(i)
		}
	}
	return regNoError
}

// buildCharclassOp is build_charclass_op (\w \W \s \S).
func (p *parser) buildCharclassOp(name, extra string, nonMatch bool) *tree {
	d := p.dfa
	sbcset := &bitset{}
	mbcset := &charset{nonMatch: nonMatch}
	buildCharclass(sbcset, mbcset, name, 0) // "we don't care the syntax"
	for i := 0; i < len(extra); i++ {
		sbcset.set(int(extra[i]))
	}
	if nonMatch {
		sbcset.not()
	}
	if d.mbCurMax > 1 {
		sbcset.mask(d.sbChar)
	}
	tr := p.createTokenTree(nil, nil, &token{typ: simpleBracket, sbcset: sbcset})
	if d.mbCurMax > 1 {
		d.hasMBNode = true
		mbc := p.createTokenTree(nil, nil, &token{typ: complexBracket, mbcset: mbcset})
		return p.createTree(tr, mbc, opAlt)
	}
	return tr
}

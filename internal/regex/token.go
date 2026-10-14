package regex

// tokType is re_token_type_t (regex_internal.h), values included: the
// epsilon bit and the order of node types matter to the port.
type tokType uint8

const (
	nonType        tokType = 0
	character      tokType = 1
	endOfRE        tokType = 2
	simpleBracket  tokType = 3
	opBackRef      tokType = 4
	opPeriod       tokType = 5
	complexBracket tokType = 6
	opUTF8Period   tokType = 7

	epsilonBit    tokType = 8
	opOpenSubexp  tokType = epsilonBit | 0
	opCloseSubexp tokType = epsilonBit | 1
	opAlt         tokType = epsilonBit | 2
	opDupAsterisk tokType = epsilonBit | 3
	anchor        tokType = epsilonBit | 4

	concat tokType = 16
	subexp tokType = 17
)

const (
	opDupPlus tokType = iota + 18
	opDupQuestion
	opOpenBracket
	opCloseBracket
	opCharsetRange
	opOpenDupNum
	opCloseDupNum
	opNonMatchList
	opOpenCollElem
	opCloseCollElem
	opOpenEquivClass
	opCloseEquivClass
	opOpenCharClass
	opCloseCharClass
	opWord
	opNotWord
	opSpace
	opNotSpace
	backSlash
)

func isEpsilon(t tokType) bool { return t&epsilonBit != 0 }

// Constraints and anchor context types (regex_internal.h).
const (
	prevWordConstraint     = 0x0001
	prevNotWordConstraint  = 0x0002
	nextWordConstraint     = 0x0004
	nextNotWordConstraint  = 0x0008
	prevNewlineConstraint  = 0x0010
	nextNewlineConstraint  = 0x0020
	prevBegbufConstraint   = 0x0040
	nextEndbufConstraint   = 0x0080
	wordDelimConstraint    = 0x0100
	notWordDelimConstraint = 0x0200

	insideWord    = prevWordConstraint | nextWordConstraint
	wordFirst     = prevNotWordConstraint | nextWordConstraint
	wordLast      = prevWordConstraint | nextNotWordConstraint
	insideNotWord = prevNotWordConstraint | nextNotWordConstraint
	lineFirst     = prevNewlineConstraint
	lineLast      = nextNewlineConstraint
	bufFirst      = prevBegbufConstraint
	bufLast       = nextEndbufConstraint
	wordDelim     = wordDelimConstraint
	notWordDelim  = notWordDelimConstraint
)

func notSatisfyPrev(constraint, context int) bool {
	return constraint&prevWordConstraint != 0 && context&contextWord == 0 ||
		constraint&prevNotWordConstraint != 0 && context&contextWord != 0 ||
		constraint&prevNewlineConstraint != 0 && context&contextNewline == 0 ||
		constraint&prevBegbufConstraint != 0 && context&contextBegbuf == 0
}

func notSatisfyNext(constraint, context int) bool {
	return constraint&nextWordConstraint != 0 && context&contextWord == 0 ||
		constraint&nextNotWordConstraint != 0 && context&contextWord != 0 ||
		constraint&nextNewlineConstraint != 0 && context&contextNewline == 0 ||
		constraint&nextEndbufConstraint != 0 && context&contextEndbuf == 0
}

// token is re_token_t. glibc keeps c, idx and ctx_type in one union; a
// back-reference or anchor token therefore also reads as the low byte of
// its index or context type, which setIdx and setCtx reproduce.
type token struct {
	typ        tokType
	c          byte
	sbcset     *bitset
	mbcset     *charset
	idx        int
	ctx        int
	constraint int
	duplicated bool
	optSubexp  bool
	acceptMB   bool
	mbPartial  bool
	wordChar   bool
}

func (t *token) setIdx(i int) { t.idx, t.c = i, byte(i) }
func (t *token) setCtx(c int) { t.ctx, t.c = c, byte(c) }

// peekToken is regcomp.c peek_token; it returns the token's length.
func (p *parser) peekToken(t *token, syn Syntax) int {
	in := p.in
	if in.eoi() {
		t.typ = endOfRE
		return 0
	}
	c := in.peekByte(0)
	t.c = c
	t.wordChar, t.mbPartial = false, false
	if in.mbCurMax > 1 && !in.firstByte(in.cur) {
		t.typ = character
		t.mbPartial = true
		return 1
	}
	if c == '\\' {
		if in.cur+1 >= in.len {
			t.typ = backSlash
			return 1
		}
		c2 := in.peekByteCase(1)
		t.c = c2
		t.typ = character
		if in.mbCurMax > 1 {
			t.wordChar = isWideWordChar(in.wcharAt(in.cur + 1))
		} else {
			t.wordChar = isWordByte(c2)
		}
		gnu := syn&NoGNUOps == 0
		switch c2 {
		case '|':
			if syn&LimitedOps == 0 && syn&NoBkVbar == 0 {
				t.typ = opAlt
			}
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			if syn&NoBkRefs == 0 {
				t.typ = opBackRef
				t.setIdx(int(c2 - '1'))
			}
		case '<':
			if gnu {
				t.typ = anchor
				t.setCtx(wordFirst)
			}
		case '>':
			if gnu {
				t.typ = anchor
				t.setCtx(wordLast)
			}
		case 'b':
			if gnu {
				t.typ = anchor
				t.setCtx(wordDelim)
			}
		case 'B':
			if gnu {
				t.typ = anchor
				t.setCtx(notWordDelim)
			}
		case 'w':
			if gnu {
				t.typ = opWord
			}
		case 'W':
			if gnu {
				t.typ = opNotWord
			}
		case 's':
			if gnu {
				t.typ = opSpace
			}
		case 'S':
			if gnu {
				t.typ = opNotSpace
			}
		case '`':
			if gnu {
				t.typ = anchor
				t.setCtx(bufFirst)
			}
		case '\'':
			if gnu {
				t.typ = anchor
				t.setCtx(bufLast)
			}
		case '(':
			if syn&NoBkParens == 0 {
				t.typ = opOpenSubexp
			}
		case ')':
			if syn&NoBkParens == 0 {
				t.typ = opCloseSubexp
			}
		case '+':
			if syn&LimitedOps == 0 && syn&BkPlusQM != 0 {
				t.typ = opDupPlus
			}
		case '?':
			if syn&LimitedOps == 0 && syn&BkPlusQM != 0 {
				t.typ = opDupQuestion
			}
		case '{':
			if syn&Intervals != 0 && syn&NoBkBraces == 0 {
				t.typ = opOpenDupNum
			}
		case '}':
			if syn&Intervals != 0 && syn&NoBkBraces == 0 {
				t.typ = opCloseDupNum
			}
		}
		return 2
	}
	t.typ = character
	if in.mbCurMax > 1 {
		t.wordChar = isWideWordChar(in.wcharAt(in.cur))
	} else {
		t.wordChar = isWordByte(t.c)
	}
	switch c {
	case '\n':
		if syn&NewlineAlt != 0 {
			t.typ = opAlt
		}
	case '|':
		if syn&LimitedOps == 0 && syn&NoBkVbar != 0 {
			t.typ = opAlt
		}
	case '*':
		t.typ = opDupAsterisk
	case '+':
		if syn&LimitedOps == 0 && syn&BkPlusQM == 0 {
			t.typ = opDupPlus
		}
	case '?':
		if syn&LimitedOps == 0 && syn&BkPlusQM == 0 {
			t.typ = opDupQuestion
		}
	case '{':
		if syn&Intervals != 0 && syn&NoBkBraces != 0 {
			t.typ = opOpenDupNum
		}
	case '}':
		if syn&Intervals != 0 && syn&NoBkBraces != 0 {
			t.typ = opCloseDupNum
		}
	case '(':
		if syn&NoBkParens != 0 {
			t.typ = opOpenSubexp
		}
	case ')':
		if syn&NoBkParens != 0 {
			t.typ = opCloseSubexp
		}
	case '[':
		t.typ = opOpenBracket
	case '.':
		t.typ = opPeriod
	case '^':
		if syn&(ContextIndepAnchors|CaretAnchorsHere) == 0 && in.cur != 0 {
			if syn&NewlineAlt == 0 || in.peekByte(-1) != '\n' {
				break
			}
		}
		t.typ = anchor
		t.setCtx(lineFirst)
	case '$':
		if syn&ContextIndepAnchors == 0 && in.cur+1 != in.len {
			var next token
			in.cur++
			p.peekToken(&next, syn)
			in.cur--
			if next.typ != opAlt && next.typ != opCloseSubexp {
				break
			}
		}
		t.typ = anchor
		t.setCtx(lineLast)
	}
	return 1
}

// isWordByte is IS_WORD_CHAR: isalnum or '_' in a single-byte locale.
func isWordByte(c byte) bool { return c == '_' || isClass("alnum", int(c)) }

func (p *parser) fetchToken(t *token, syn Syntax) { p.in.cur += p.peekToken(t, syn) }

// peekTokenBracket is peek_token_bracket.
func (p *parser) peekTokenBracket(t *token, syn Syntax) int {
	in := p.in
	if in.eoi() {
		t.typ = endOfRE
		return 0
	}
	c := in.peekByte(0)
	t.c = c
	if in.mbCurMax > 1 && !in.firstByte(in.cur) {
		t.typ = character
		return 1
	}
	if c == '\\' && syn&BackslashEscapeInLists != 0 && in.cur+1 < in.len {
		in.cur++ // '\' escapes the next byte
		t.c = in.peekByte(0)
		t.typ = character
		return 1
	}
	if c == '[' {
		var c2 byte
		if in.cur+1 < in.len {
			c2 = in.peekByte(1)
		}
		t.c = c2
		switch {
		case c2 == '.':
			t.typ = opOpenCollElem
			return 2
		case c2 == '=':
			t.typ = opOpenEquivClass
			return 2
		case c2 == ':' && syn&CharClasses != 0:
			t.typ = opOpenCharClass
			return 2
		}
		t.typ = character
		t.c = c
		return 1
	}
	switch c {
	case ']':
		t.typ = opCloseBracket
	case '^':
		t.typ = opNonMatchList
	case '-':
		// "---" is one '-' (V7 grep and awk)
		if !(in.cur+2 < in.len && in.peekByte(1) == '-' && in.peekByte(2) == '-') {
			t.typ = opCharsetRange
			break
		}
		in.cur += 2
		t.typ = character
	default:
		t.typ = character
	}
	return 1
}

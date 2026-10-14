package datetime

import "strings"

// lexer is parse-datetime.y yylex over the parser state.
type lexer struct {
	pc  *parserControl
	in  string
	pos int
}

// ctl is the grammar actions' access to parser_control *pc.
func ctl(l yyLexer) *parserControl { return l.(*lexer).pc }

func (l *lexer) Error(string) {} // yyerror does nothing

// at is *p on the NUL-terminated C string: 0 past the end.
func (l *lexer) at(i int) byte {
	if i < len(l.in) {
		return l.in[i]
	}
	return 0
}

func cIsSpace(c byte) bool { return c == ' ' || (c >= '\t' && c <= '\r') }
func cIsDigit(c byte) bool { return c >= '0' && c <= '9' }
func cIsAlpha(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }

// Lex is parse-datetime.y yylex.
func (l *lexer) Lex(lval *yySymType) int {
	pc := l.pc
	for {
		for cIsSpace(l.at(l.pos)) {
			l.pos++
		}
		c := l.at(l.pos)
		if cIsDigit(c) || c == '-' || c == '+' {
			p, sign := l.pos, 0
			if c == '-' || c == '+' {
				sign = 1
				if c == '-' {
					sign = -1
				}
				for {
					p++
					l.pos = p
					if c = l.at(p); !cIsSpace(c) {
						break
					}
				}
				if !cIsDigit(c) {
					continue // a lone sign is skipped
				}
			}
			var value int64
			for {
				v, ovf1 := ckdMul(value, 10)
				d := int64(c - '0')
				if sign < 0 {
					d = -d
				}
				v, ovf2 := ckdAdd(v, d)
				if ovf1 || ovf2 {
					return '?'
				}
				value = v
				p++
				if c = l.at(p); !cIsDigit(c) {
					break
				}
			}
			if (c == '.' || c == ',') && cIsDigit(l.at(p+1)) {
				if pc.notDecimal { // the second period of DD.MM.YYYY
					pc.notDecimal = false
					lval.textint = textint{negative: sign < 0, value: value, digits: int64(p - l.pos)}
					l.pos = p
					return tUNUMBER_DOTTED
				}
				ts, end, ok := l.fraction(value, sign, p)
				if ok {
					lval.ts = ts
					l.pos = end
					if sign != 0 {
						return tSDECIMAL_NUMBER
					}
					return tUDECIMAL_NUMBER
				}
				if end < 0 {
					return '?'
				}
				pc.notDecimal = true // a DD.MM.YYYY date, not a decimal: normal_value
			} else {
				pc.notDecimal = false
			}
			lval.textint = textint{negative: sign < 0, value: value, digits: int64(p - l.pos)}
			l.pos = p
			if sign != 0 {
				return tSNUMBER
			}
			return tUNUMBER
		}
		if cIsAlpha(c) {
			var word []byte
			for {
				if len(word) < 19 { // char buff[20]
					word = append(word, c)
				}
				l.pos++
				if c = l.at(l.pos); !(cIsAlpha(c) || c == '.') {
					break
				}
			}
			tp := pc.lookupWord(string(word))
			if tp == nil {
				return '?'
			}
			lval.intval = tp.value
			return tp.typ
		}
		if c != '(' {
			l.pos++
			return int(c) // 0 at the end of the input: EOF
		}
		for count := 0; ; { // skip a parenthesised comment
			c = l.at(l.pos)
			l.pos++
			if c == 0 {
				return 0
			}
			if c == '(' {
				count++
			} else if c == ')' {
				count--
			}
			if count == 0 {
				break
			}
		}
	}
}

// fraction accumulates the digits after '.' or ',' at p to nanosecond
// precision (yylex's decimal branch). ok is false with end >= 0 when a
// second '.' shows a DD.MM.YYYY date, and with end < 0 on overflow.
func (l *lexer) fraction(s int64, sign, p int) (ts timespec, end int, ok bool) {
	p++
	ns := int64(l.at(p) - '0')
	p++
	for digits := 2; digits <= 9; digits++ {
		ns *= 10
		if l.at(p) == '.' {
			return timespec{}, 0, false
		}
		if cIsDigit(l.at(p)) {
			ns += int64(l.at(p) - '0')
			p++
		}
	}
	if sign < 0 { // skip excess digits, truncating toward -Infinity
		for ; cIsDigit(l.at(p)); p++ {
			if l.at(p) != '0' {
				ns++
				break
			}
		}
	}
	for cIsDigit(l.at(p)) {
		p++
	}
	if sign < 0 && ns != 0 {
		var ovf bool
		if s, ovf = ckdSub(s, 1); ovf {
			return timespec{}, -1, false
		}
		ns = 1e9 - ns
	}
	return timespec{sec: s, nsec: ns}, p, true
}

// lookupWord is parse-datetime.y lookup_word.
func (pc *parserControl) lookupWord(word string) *table {
	word = strings.ToUpper(word) // c_toupper: only ASCII letters and '.' reach here
	for i := range meridianTable {
		if word == meridianTable[i].name {
			return &meridianTable[i]
		}
	}
	abbrev := len(word) == 3 || (len(word) == 4 && word[3] == '.')
	for i := range monthAndDayTable {
		name := monthAndDayTable[i].name
		// strncmp(word, name, 3): every name is longer than 3 characters.
		if (abbrev && strings.HasPrefix(name, word[:3])) || (!abbrev && word == name) {
			return &monthAndDayTable[i]
		}
	}
	if tp := pc.lookupZone(word); tp != nil {
		return tp
	}
	if word == dstTable.name {
		return &dstTable
	}
	for i := range timeUnitsTable {
		if word == timeUnitsTable[i].name {
			return &timeUnitsTable[i]
		}
	}
	if singular, ok := strings.CutSuffix(word, "S"); ok { // strip a plural
		for i := range timeUnitsTable {
			if singular == timeUnitsTable[i].name {
				return &timeUnitsTable[i]
			}
		}
	}
	for i := range relativeTimeTable {
		if word == relativeTimeTable[i].name {
			return &relativeTimeTable[i]
		}
	}
	if len(word) == 1 {
		for i := range militaryTable {
			if word[0] == militaryTable[i].name[0] {
				return &militaryTable[i]
			}
		}
	}
	if strings.Contains(word, ".") { // drop periods and try the zones again
		return pc.lookupZone(strings.ReplaceAll(word, ".", ""))
	}
	return nil
}

// lookupZone is lookup_zone: universal zones, then the local zone's own
// abbreviations, then the fixed table.
func (pc *parserControl) lookupZone(name string) *table {
	for i := range universalTimeZoneTable {
		if universalTimeZoneTable[i].name == name {
			return &universalTimeZoneTable[i]
		}
	}
	for i := 0; pc.localTimeZoneTable[i].name != ""; i++ { // a NULL name ends it
		if pc.localTimeZoneTable[i].name == name {
			return &pc.localTimeZoneTable[i]
		}
	}
	for i := range timeZoneTable {
		if timeZoneTable[i].name == name {
			return &timeZoneTable[i]
		}
	}
	return nil
}

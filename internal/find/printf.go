package find

import "strings"

type segKind uint8

const (
	segPlain  segKind = iota // KIND_PLAIN
	segStop                  // KIND_STOP
	segFormat                // KIND_FORMAT
)

// segment is struct segment (print.c): literal text, or a directive with
// its literal prefix, C spec and conversion.
type segment struct {
	kind segKind
	text string // plain/stop text, or the directive's literal prefix
	spec cspec
	raw  string // prefix + "%" + spec text: what %% fwrites (text_len bytes)
	conv byte   // 's', 'd', 'o', 'g' or '%'
	fc   byte   // format_char[0]
	fc2  byte   // format_char[1] for A, B, C, T
}

// formatVal is struct format_val: the destination and the segments.
type formatVal struct {
	out      *Stream
	segments []segment
}

// insertFprintf is print.c insert_fprintf. Segment boundaries follow GNU
// exactly: each segment is one checked stdio call, which decides how
// often a failing stream is diagnosed.
func (ps *parser) insertFprintf(out *Stream, e *parserEntry, format string) bool {
	r := ps.r
	p := ps.insertPrimaryWithPred(e, predFprintf, fnFprintf, format, true)
	p.sideEffects, p.noDefaultPrint = true, true
	p.needType, p.needStat = false, false
	p.cost = NeedsNothing
	fv := &formatVal{out: out}
	p.args.format = fv

	b := []byte(format) // escapes are decoded in place, as GNU does
	at := func(i int) byte {
		if i < len(b) {
			return b[i]
		}
		return 0 // the terminating NUL
	}
	start := 0 // segstart
	plain := func(kind segKind, end int) {
		fv.segments = append(fv.segments, segment{kind: kind, text: string(b[start:end])})
	}
	i := 0
	for ; i < len(b); i++ {
		switch {
		case b[i] == '\\' && at(i+1) == 'c':
			// A mid-evaluation flush: evaluation goes serial (needsSerial).
			r.formatFlushes = true
			plain(segStop, i)
			if p.needStat && p.cost < NeedsStatInfo {
				p.cost = NeedsStatInfo
			}
			return true
		case b[i] == '\\':
			readpos := 1
			switch c := at(i + 1); {
			case c == 0:
				r.errorf(nil, "warning: escape `\\' followed by nothing at all")
				readpos = 0 // the backslash itself is the result
			case c >= '0' && c <= '7':
				n, k := 0, 0
				for ; k < 3 && at(i+1+k) >= '0' && at(i+1+k) <= '7'; k++ {
					n = 8*n + int(b[i+1+k]-'0')
				}
				b[i] = byte(n)
				readpos = k
			default:
				val := escapeChar(c)
				if val == 0 {
					// The backslash and the character stay in the text.
					r.errorf(nil, "warning: unrecognized escape `\\%s'", cbyte(c))
					i += readpos
					continue
				}
				b[i] = val
			}
			plain(segPlain, i+1)
			start = i + readpos + 1
			i += readpos
		case b[i] == '%':
			if at(i+1) == 0 {
				r.fatalf(nil, "error: %s at end of format string", string(b[i:]))
			}
			pos := i + 1
			if b[i+1] != '%' {
				pos = i + formatFlagsLength(b[i:])
			}
			dc := at(pos)
			n := specifierLength(dc)
			if n > 0 && at(pos+n-1) != 0 {
				seg := segment{kind: segFormat, text: string(b[start:i]), raw: string(b[start:pos]),
					spec: parseCspec(string(b[i+1 : pos])), fc: dc}
				if n == 2 {
					seg.fc2 = b[pos+1]
				}
				makeSegment(&seg, p)
				fv.segments = append(fv.segments, seg)
				pos += n - 1
			} else {
				// strchr("{[(", 0) matches the terminator: "%-5" at the end of
				// the format is "reserved" too, with a raw NUL in the message.
				if dc == 0 || strings.IndexByte("{[(", dc) >= 0 {
					r.fatalf(nil, "error: the format directive `%%%s' is reserved for future use", cbyte(dc))
				}
				if n == 2 && at(pos+1) == 0 {
					r.errorf(nil, "warning: format directive `%%%s' should be followed by another character", cbyte(dc))
				} else {
					r.errorf(nil, "warning: unrecognized format directive `%%%s'", cbyte(dc))
				}
				plain(segPlain, pos+1)
			}
			start = pos + 1
			i = pos
		}
	}
	if i > start {
		plain(segPlain, i)
	}
	return true
}

// escapeChar is print.c parse_escape_char: 0 for an unknown escape.
func escapeChar(c byte) byte {
	if i := strings.IndexByte("abfnrtv\\", c); i >= 0 {
		return "\a\b\f\n\r\t\v\\"[i]
	}
	return 0
}

// formatFlagsLength is print.c get_format_flags_length.
func formatFlagsLength(p []byte) int {
	n := 1
	for n < len(p) && strings.IndexByte("-+ #", p[n]) >= 0 {
		n++
	}
	for n < len(p) && p[n] >= '0' && p[n] <= '9' {
		n++
	}
	if n < len(p) && p[n] == '.' {
		for n++; n < len(p) && p[n] >= '0' && p[n] <= '9'; n++ {
		}
	}
	return n
}

// specifierLength is print.c get_format_specifer_length. Like strchr, it
// finds the terminating NUL in its set, so 0 has length 1.
func specifierLength(c byte) int {
	switch {
	case c == 0 || strings.IndexByte("abcdDfFgGhHiklmMnpPsStuUyYZ%", c) >= 0:
		return 1
	case strings.IndexByte("ABCT", c) >= 0:
		return 2
	}
	return 0
}

// makeSegment is print.c make_segment for KIND_FORMAT: the conversion
// letter, the predicate's needs and its cost.
func makeSegment(s *segment, p *Predicate) {
	my := NeedsNothing
	s.conv = 's'
	switch s.fc {
	case '%':
		s.conv = '%'
	case 'l':
		p.needStat, my = true, NeedsLinkName
	case 'y':
		p.needType, my = true, NeedsType
	case 'i':
		p.needInum, my = true, NeedsInodeNumber
	case 'a', 'A', 'B', 'c', 'C', 'F', 'g', 'M', 's', 't', 'T', 'u', 'G', 'U', 'b', 'D', 'k', 'n':
		p.needStat, my = true, NeedsStatInfo
	case 'S':
		p.needStat, my, s.conv = true, NeedsStatInfo, 'g'
	case 'Y':
		p.needStat, my = true, NeedsType
	case 'Z':
		my = NeedsAccessInfo
	case 'd':
		s.conv = 'd'
	case 'm':
		s.conv, p.needStat, my = 'o', true, NeedsStatInfo
	}
	if my > p.cost {
		p.cost = my
	}
}

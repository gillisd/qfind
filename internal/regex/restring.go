package regex

// weof is WEOF in the wide-character buffer: a byte that continues the
// character started before it.
const weof rune = -1

// reString is regex_internal.c's re_string_t, built in one go over the
// whole string (glibc builds it incrementally, with the same result).
type reString struct {
	raw           []byte // raw_mbs
	mbs           []byte // the string, case-folded under RE_ICASE
	wcs           []rune // mb_cur_max > 1: the character at each byte, or weof
	offsets       []int  // raw index of each mbs byte, when offsetsNeeded
	offsetsNeeded bool
	mbsAllocated  bool
	mbCurMax      int
	cur           int // cur_idx
	len           int // len (= stop = valid_len once built)
	rawLen        int // valid_raw_len
	tipContext    int
	wordChar      *bitset
	wordOpsUsed   bool
}

// newReString is re_string_construct / re_string_allocate followed by
// building every buffer: build_wcs_buffer, build_wcs_upper_buffer or
// build_upper_buffer.
func newReString(s []byte, icase bool, mbCurMax int) *reString {
	p := &reString{raw: s, mbs: s, mbCurMax: mbCurMax, mbsAllocated: icase,
		len: len(s), rawLen: len(s), tipContext: contextNewline | contextBegbuf}
	switch {
	case mbCurMax > 1 && icase:
		p.buildWcsUpper()
	case mbCurMax > 1:
		p.buildWcs()
	case icase:
		p.mbs = make([]byte, len(s))
		for i, c := range s {
			p.mbs[i] = toupperC(c)
		}
	}
	return p
}

// toupperC is toupper in a single-byte locale, which for find is C.
func toupperC(c byte) byte {
	if 'a' <= c && c <= 'z' {
		return c - 'a' + 'A'
	}
	return c
}

// buildWcs is build_wcs_buffer: an invalid or incomplete sequence is a
// single-byte character whose wide value is the byte itself.
func (p *reString) buildWcs() {
	p.wcs = make([]rune, len(p.raw))
	for i := 0; i < len(p.raw); {
		wc, n := decodeChar(p.raw, i)
		if n == 0 {
			wc, n = rune(p.raw[i]), 1
		}
		p.wcs[i] = wc
		for k := 1; k < n; k++ {
			p.wcs[i+k] = weof
		}
		i += n
	}
}

// buildWcsUpper is build_wcs_upper_buffer: each valid character is
// replaced by towupper's, recording raw offsets once a replacement changes
// the length.
func (p *reString) buildWcsUpper() {
	raw := p.raw
	p.mbs = make([]byte, 0, len(raw))
	p.wcs = make([]rune, 0, len(raw))
	p.offsets = make([]int, 0, len(raw))
	for src := 0; src < len(raw); {
		wc, n := decodeChar(raw, src)
		if n == 0 {
			p.mbs = append(p.mbs, raw[src])
			p.wcs = append(p.wcs, rune(raw[src]))
			p.offsets = append(p.offsets, src)
			src++
			continue
		}
		wcu := towupper(wc)
		var enc []byte
		if wcu != wc {
			enc = appendChar(nil, wcu)
		} else {
			enc = raw[src : src+n]
		}
		if len(enc) != n {
			p.offsetsNeeded = true
		}
		for k := range enc {
			p.mbs = append(p.mbs, enc[k])
			if k == 0 {
				p.wcs = append(p.wcs, wcu)
			} else {
				p.wcs = append(p.wcs, weof)
			}
			p.offsets = append(p.offsets, src+min(k, n-1))
		}
		src += n
	}
	p.len = len(p.mbs)
}

func (p *reString) eoi() bool             { return p.len <= p.cur }
func (p *reString) peekByte(off int) byte { return p.mbs[p.cur+off] }
func (p *reString) fetchByte() byte {
	c := p.mbs[p.cur]
	p.cur++
	return c
}

// firstByte is re_string_first_byte.
func (p *reString) firstByte(idx int) bool { return idx == p.len || p.wcs[idx] != weof }

// isSingleByteChar is re_string_is_single_byte_char.
func (p *reString) isSingleByteChar(idx int) bool {
	return p.wcs[idx] != weof && (p.len == idx+1 || p.wcs[idx+1] != weof)
}

// peekByteCase is re_string_peek_byte_case: the raw byte, except that a
// multibyte character (or a non-ASCII byte once offsets are needed) is
// read folded.
func (p *reString) peekByteCase(off int) byte {
	if !p.mbsAllocated {
		return p.peekByte(off)
	}
	if p.mbCurMax > 1 && !p.isSingleByteChar(p.cur+off) {
		return p.peekByte(off)
	}
	o := p.cur + off
	if p.offsetsNeeded {
		o = p.offsets[o]
	}
	ch := p.raw[o]
	if p.offsetsNeeded && ch >= 0x80 {
		return p.peekByte(off)
	}
	return ch
}

// fetchByteCase is re_string_fetch_byte_case.
func (p *reString) fetchByteCase() byte {
	if !p.mbsAllocated {
		return p.fetchByte()
	}
	if p.offsetsNeeded {
		if !p.firstByte(p.cur) {
			return p.fetchByte()
		}
		ch := p.raw[p.offsets[p.cur]]
		if ch >= 0x80 {
			return p.fetchByte()
		}
		p.cur += p.charSizeAt(p.cur)
		return ch
	}
	c := p.raw[p.cur]
	p.cur++
	return c
}

// charSizeAt is re_string_char_size_at.
func (p *reString) charSizeAt(idx int) int {
	if p.mbCurMax == 1 {
		return 1
	}
	n := 1
	for ; idx+n < p.len; n++ {
		if p.wcs[idx+n] != weof {
			break
		}
	}
	return n
}

// wcharAt is re_string_wchar_at.
func (p *reString) wcharAt(idx int) rune {
	if p.mbCurMax == 1 {
		return rune(p.mbs[idx])
	}
	return p.wcs[idx]
}

// Contexts (regex_internal.h).
const (
	contextWord    = 1
	contextNewline = 2
	contextBegbuf  = 4
	contextEndbuf  = 8
)

// contextAt is re_string_context_at with newline_anchor set and no
// REG_NOTBOL/REG_NOTEOL.
func (p *reString) contextAt(idx int) int {
	if idx < 0 {
		return p.tipContext
	}
	if idx == p.len {
		return contextNewline | contextEndbuf
	}
	if p.mbCurMax > 1 {
		i := idx
		for p.wcs[i] == weof {
			i--
			if i < 0 {
				return p.tipContext
			}
		}
		wc := p.wcs[i]
		if p.wordOpsUsed && isWideWordChar(wc) {
			return contextWord
		}
		if wc == '\n' {
			return contextNewline
		}
		return 0
	}
	c := int(p.mbs[idx])
	if p.wordChar.contains(c) {
		return contextWord
	}
	if c == '\n' {
		return contextNewline
	}
	return 0
}

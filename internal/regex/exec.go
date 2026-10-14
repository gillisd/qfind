package regex

import "sync"

// Regexp is a compiled pattern: re_compile_pattern's result.
type Regexp struct {
	d     *dfa
	icase bool
	pool  sync.Pool // of *cache
}

// Compile is re_compile_pattern (newline_anchor = 1) for a pattern string;
// utf8 selects a UTF-8 locale (C.UTF-8 or *.UTF-8) over the C locale. A
// compile error's Error() is glibc's message.
func Compile(pattern string, syn Syntax, utf8 bool) (*Regexp, error) {
	d := &dfa{mbCurMax: 1}
	if utf8 {
		d.mbCurMax, d.isUTF8, d.sbChar = 6, true, &utf8SBMap
	}
	icase := syn&ICase != 0
	p := &parser{in: newReString([]byte(pattern), icase, d.mbCurMax), dfa: d}
	d.strTree = p.parse(syn)
	if d.strTree == nil {
		return nil, &Error{p.err}
	}
	if err := d.analyze(p); err != regNoError {
		return nil, &Error{err}
	}
	if d.isUTF8 && !icase {
		d.optimizeUTF8()
	}
	d.createInitialNodes()
	d.strTree = nil
	re := &Regexp{d: d, icase: icase}
	re.pool.New = func() any { return newCache(d) }
	return re, nil
}

// FullMatch is re_match(re, s, len, 0, NULL) == len: the longest match
// that starts at 0 spans all of s.
func (re *Regexp) FullMatch(s string) bool {
	c := re.pool.Get().(*cache)
	defer re.pool.Put(c)
	in := newReString([]byte(s), re.icase, re.d.mbCurMax)
	in.wordChar, in.wordOpsUsed = &re.d.wordChar, re.d.wordOpsUsed
	m := &matcher{c: c, d: re.d, in: in}
	return m.search() == in.len
}

// matcher is re_match_context_t.
type matcher struct {
	c         *cache
	d         *dfa
	in        *reString
	matchLast int
	lastNode  int
	stateLog  []*state
	logTop    int
	bkrefEnts []bkrefEntry
	maxMBElem int
	subTops   []*subTop
}

// search is re_search_internal for re_match with no registers (nmatch is
// 1, so fl_longest_match is set and set_regs never runs): the length of
// the longest match starting at 0, or -1.
func (m *matcher) search() int {
	d := m.d
	if d.hasMBNode {
		m.stateLog = make([]*state, m.in.len+1)
	}
	matchLast := m.checkMatching()
	if matchLast < 0 {
		return -1
	}
	m.matchLast = matchLast
	if d.nbackref > 0 {
		m.lastNode = m.checkHaltStateContext(m.stateLog[matchLast], matchLast)
		if !m.pruneImpossibleNodes() {
			return -1
		}
	}
	return m.matchLast
}

// acquireInitStateContext is acquire_init_state_context at index 0.
func (m *matcher) acquireInitStateContext(idx int) *state {
	c := m.c
	if !c.initState.hasConstraint {
		return c.initState
	}
	ctx := m.in.contextAt(idx - 1)
	switch {
	case ctx&contextWord != 0:
		return c.initWord
	case ctx == 0:
		return c.initState
	case ctx&contextBegbuf != 0 && ctx&contextNewline != 0:
		return c.initBegbuf
	case ctx&contextNewline != 0:
		return c.initNL
	case ctx&contextBegbuf != 0:
		return c.acquireContext(c.initState.entrance, ctx)
	}
	return c.initState
}

// checkMatching is check_matching with fl_longest_match: the end of the
// longest match, or -1.
func (m *matcher) checkMatching() int {
	d, in := m.d, m.in
	matchLast := -1
	cur := m.acquireInitStateContext(in.cur)
	if m.stateLog != nil {
		m.stateLog[in.cur] = cur
		if d.nbackref > 0 {
			m.checkSubexpMatchingTop(cur.nodes, 0)
			if cur.hasBackref {
				m.transitStateBkref(cur.nodes)
			}
		}
	}
	if cur.halt && (!cur.hasConstraint || m.checkHaltStateContext(cur, in.cur) != 0) {
		matchLast = in.cur
	}
	for !in.eoi() {
		cur = m.transitState(cur)
		if m.stateLog != nil {
			cur = m.mergeStateWithLog(cur)
		}
		if cur == nil {
			if m.stateLog == nil {
				break
			}
			if cur = m.findRecoverState(); cur == nil {
				break
			}
		}
		if cur.halt && (!cur.hasConstraint || m.checkHaltStateContext(cur, in.cur) != 0) {
			matchLast = in.cur
		}
	}
	return matchLast
}

// checkHaltNodeContext is check_halt_node_context.
func (m *matcher) checkHaltNodeContext(node, context int) bool {
	n := &m.d.nodes[node]
	return n.typ == endOfRE && (n.constraint == 0 || !notSatisfyNext(n.constraint, context))
}

// checkHaltStateContext is check_halt_state_context: the halt node, or 0.
func (m *matcher) checkHaltStateContext(s *state, idx int) int {
	ctx := m.in.contextAt(idx)
	for _, n := range s.nodes {
		if m.checkHaltNodeContext(n, ctx) {
			return n
		}
	}
	return 0
}

// transitState is transit_state.
func (m *matcher) transitState(s *state) *state {
	if s.acceptMB {
		m.transitStateMB(s)
	}
	in := m.in
	ch := in.fetchByte()
	if s.trtable == nil && s.wordTrtable == nil {
		m.buildTrtable(s)
	}
	if s.trtable != nil {
		return s.trtable[ch]
	}
	if in.contextAt(in.cur-1)&contextWord != 0 {
		return s.wordTrtable[int(ch)+256]
	}
	return s.wordTrtable[ch]
}

// mergeStateWithLog is merge_state_with_log.
func (m *matcher) mergeStateWithLog(next *state) *state {
	d, in := m.d, m.in
	cur := in.cur
	switch {
	case cur > m.logTop:
		m.stateLog[cur] = next
		m.logTop = cur
	case m.stateLog[cur] == nil:
		m.stateLog[cur] = next
	default:
		// a multibyte character or back-reference also ends here: the
		// next state is the union of both
		logNodes := m.stateLog[cur].entrance
		var nextNodes nodeSet
		if next != nil && next.entrance != nil {
			nextNodes = union(next.entrance, logNodes)
		} else {
			nextNodes = logNodes
		}
		ctx := in.contextAt(in.cur - 1)
		next = m.c.acquireContext(nextNodes, ctx)
		m.stateLog[cur] = next
	}
	if d.nbackref > 0 && next != nil {
		m.checkSubexpMatchingTop(next.nodes, cur)
		if next.hasBackref {
			m.transitStateBkref(next.nodes)
			next = m.stateLog[cur]
		}
	}
	return next
}

// findRecoverState is find_recover_state.
func (m *matcher) findRecoverState() *state {
	for {
		max := m.logTop
		cur := m.in.cur
		for {
			cur++
			if cur > max {
				return nil
			}
			m.in.cur++
			if m.stateLog[cur] != nil {
				break
			}
		}
		if s := m.mergeStateWithLog(nil); s != nil {
			return s
		}
	}
}

// checkSubexpMatchingTop is check_subexp_matching_top.
func (m *matcher) checkSubexpMatchingTop(nodes nodeSet, strIdx int) {
	d := m.d
	for _, n := range nodes {
		t := &d.nodes[n]
		if t.typ == opOpenSubexp && t.idx < 64 && d.usedBkrefMap&(1<<t.idx) != 0 {
			m.subTops = append(m.subTops, &subTop{strIdx: strIdx, node: n})
		}
	}
}

// transitStateMB is transit_state_mb.
func (m *matcher) transitStateMB(s *state) {
	d, in := m.d, m.in
	for _, cur := range s.nodes {
		n := &d.nodes[cur]
		if !n.acceptMB {
			continue
		}
		if n.constraint != 0 && notSatisfyNext(n.constraint, in.contextAt(in.cur)) {
			continue
		}
		k := m.checkNodeAcceptBytes(cur, in.cur)
		if k == 0 {
			continue
		}
		dest := in.cur + k
		m.maxMBElem = max(m.maxMBElem, k)
		m.cleanStateLogIfNeeded(dest)
		newNodes := d.eclosures[d.nexts[cur]]
		destNodes := newNodes
		if ds := m.stateLog[dest]; ds != nil {
			destNodes = union(ds.entrance, newNodes)
		}
		m.stateLog[dest] = m.c.acquireContext(destNodes, in.contextAt(dest-1))
	}
}

// cleanStateLogIfNeeded is clean_state_log_if_needed.
func (m *matcher) cleanStateLogIfNeeded(next int) {
	if m.logTop < next {
		for i := m.logTop + 1; i <= next; i++ {
			m.stateLog[i] = nil
		}
		m.logTop = next
	}
}

// buildTrtable is build_trtable (with group_nodes_into_DFAstates).
func (m *matcher) buildTrtable(s *state) {
	d, c := m.d, m.c
	destsNode, destsCh := m.groupNodes(s)
	if len(destsNode) == 0 {
		s.trtable = &[256]*state{}
		return
	}
	n := len(destsNode)
	dest := make([]*state, n)
	destWord := make([]*state, n)
	destNL := make([]*state, n)
	var acceptable bitset
	needWord := false
	for i := range destsNode {
		var follows nodeSet
		for _, e := range destsNode[i] {
			if next := d.nexts[e]; next != -1 {
				follows.merge(d.eclosures[next])
			}
		}
		dest[i] = c.acquireContext(follows, 0)
		if dest[i] != nil && dest[i].hasConstraint {
			destWord[i] = c.acquireContext(follows, contextWord)
			if dest[i] != destWord[i] && d.mbCurMax > 1 {
				needWord = true
			}
			destNL[i] = c.acquireContext(follows, contextNewline)
		} else {
			destWord[i], destNL[i] = dest[i], dest[i]
		}
		acceptable.merge(&destsCh[i])
	}
	owner := func(ch int) int {
		j := 0
		for !destsCh[j].contains(ch) {
			j++
		}
		return j
	}
	if !needWord {
		t := &[256]*state{}
		for ch := 0; ch < 256; ch++ {
			if !acceptable.contains(ch) {
				continue
			}
			j := owner(ch)
			if d.wordChar.contains(ch) {
				t[ch] = destWord[j]
			} else {
				t[ch] = dest[j]
			}
		}
		if acceptable.contains('\n') {
			t['\n'] = destNL[owner('\n')]
		}
		s.trtable = t
		return
	}
	t := &[512]*state{}
	for ch := 0; ch < 256; ch++ {
		if !acceptable.contains(ch) {
			continue
		}
		j := owner(ch)
		t[ch] = dest[j]
		t[ch+256] = destWord[j]
	}
	if acceptable.contains('\n') {
		j := owner('\n')
		t['\n'] = destNL[j]
		t['\n'+256] = destNL[j]
	}
	s.wordTrtable = t
}

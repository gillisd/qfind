package regex

import "bytes"

// stateArray is state_array_t: a state log for check_arrival, reused
// (from next_idx on) by later calls on the same path.
type stateArray struct {
	nextIdx int
	array   []*state
}

// subLast is re_sub_match_last_t: an OP_CLOSE_SUBEXP reached at strIdx.
type subLast struct {
	node, strIdx int
	path         stateArray
}

// subTop is re_sub_match_top_t: an OP_OPEN_SUBEXP met at strIdx.
type subTop struct {
	strIdx, node int
	path         *stateArray
	lasts        []*subLast
}

// bkrefEntry is struct re_backref_cache_entry.
type bkrefEntry struct {
	node, strIdx, subexpFrom, subexpTo int
	epsReachable                       uint64
	more                               bool
}

// addEntry is match_ctx_add_entry.
func (m *matcher) addEntry(node, strIdx, from, to int) {
	if n := len(m.bkrefEnts); n > 0 && m.bkrefEnts[n-1].strIdx == strIdx {
		m.bkrefEnts[n-1].more = true
	}
	e := bkrefEntry{node: node, strIdx: strIdx, subexpFrom: from, subexpTo: to}
	if from == to {
		e.epsReachable = ^uint64(0)
	}
	m.bkrefEnts = append(m.bkrefEnts, e)
	m.maxMBElem = max(m.maxMBElem, to-from)
}

// searchCurBkrefEntry is search_cur_bkref_entry: the first entry at
// strIdx, or -1.
func (m *matcher) searchCurBkrefEntry(strIdx int) int {
	lo, hi := 0, len(m.bkrefEnts)
	for lo < hi {
		mid := (lo + hi) / 2
		if m.bkrefEnts[mid].strIdx < strIdx {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(m.bkrefEnts) && m.bkrefEnts[lo].strIdx == strIdx {
		return lo
	}
	return -1
}

// transitStateBkref is transit_state_bkref.
func (m *matcher) transitStateBkref(nodes nodeSet) {
	d, in := m.d, m.in
	cur := in.cur
	for _, nodeIdx := range nodes {
		n := &d.nodes[nodeIdx]
		if n.typ != opBackRef {
			continue
		}
		if n.constraint != 0 && notSatisfyNext(n.constraint, in.contextAt(cur)) {
			continue
		}
		bkc := len(m.bkrefEnts)
		m.getSubexp(nodeIdx, cur)
		for ; bkc < len(m.bkrefEnts); bkc++ {
			ent := m.bkrefEnts[bkc]
			if ent.node != nodeIdx || ent.strIdx != cur {
				continue
			}
			subLen := ent.subexpTo - ent.subexpFrom
			var newDest nodeSet
			if subLen == 0 {
				newDest = d.eclosures[d.edests[nodeIdx][0]]
			} else {
				newDest = d.eclosures[d.nexts[nodeIdx]]
			}
			destIdx := cur + subLen
			ctx := in.contextAt(destIdx - 1)
			prevN := 0
			if s := m.stateLog[cur]; s != nil {
				prevN = len(s.nodes)
			}
			if ds := m.stateLog[destIdx]; ds == nil {
				m.stateLog[destIdx] = m.c.acquireContext(newDest, ctx)
			} else {
				m.stateLog[destIdx] = m.c.acquireContext(union(ds.entrance, newDest), ctx)
			}
			// an empty back-reference transits by epsilon: re-check
			if subLen == 0 && len(m.stateLog[cur].nodes) > prevN {
				m.checkSubexpMatchingTop(newDest, cur)
				m.transitStateBkref(newDest)
			}
		}
	}
}

// getSubexp is get_subexp: registers every candidate the back-reference
// at bkrefStrIdx might match; prune_impossible_nodes checks them later.
func (m *matcher) getSubexp(bkrefNode, bkrefStrIdx int) {
	d, buf := m.d, m.in.mbs
	if ci := m.searchCurBkrefEntry(bkrefStrIdx); ci != -1 {
		for i := ci; ; i++ {
			if m.bkrefEnts[i].node == bkrefNode {
				return // already checked
			}
			if !m.bkrefEnts[i].more {
				break
			}
		}
	}
	subexpNum := d.nodes[bkrefNode].idx
	for ti := 0; ti < len(m.subTops); ti++ {
		top := m.subTops[ti]
		if d.nodes[top.node].idx != subexpNum {
			continue
		}
		slStr := top.strIdx
		bkrefOff := bkrefStrIdx
		li := 0
		for ; li < len(top.lasts); li++ {
			last := top.lasts[li]
			diff := last.strIdx - slStr
			if diff > 0 {
				if bkrefOff+diff > m.in.len {
					break // not enough characters left
				}
				if !bytes.Equal(buf[bkrefOff:bkrefOff+diff], buf[slStr:slStr+diff]) {
					break
				}
			}
			bkrefOff += diff
			slStr += diff
			m.getSubexpSub(top, last, bkrefNode, bkrefStrIdx)
		}
		if li < len(top.lasts) {
			continue
		}
		if li > 0 {
			slStr++
		}
		for ; slStr <= bkrefStrIdx; slStr++ {
			off := slStr - top.strIdx
			if off > 0 {
				if bkrefOff >= m.in.len {
					break
				}
				if buf[bkrefOff] != buf[slStr-1] {
					break
				}
				bkrefOff++
			}
			if m.stateLog[slStr] == nil {
				continue
			}
			cls := d.findSubexpNode(m.stateLog[slStr].nodes, subexpNum, opCloseSubexp)
			if cls == -1 {
				continue
			}
			if top.path == nil {
				top.path = &stateArray{}
			}
			if !m.checkArrival(top.path, top.node, top.strIdx, cls, slStr, opCloseSubexp) {
				continue
			}
			last := &subLast{node: cls, strIdx: slStr}
			top.lasts = append(top.lasts, last)
			m.getSubexpSub(top, last, bkrefNode, bkrefStrIdx)
		}
	}
}

// getSubexpSub is get_subexp_sub.
func (m *matcher) getSubexpSub(top *subTop, last *subLast, bkrefNode, bkrefStr int) bool {
	if !m.checkArrival(&last.path, last.node, last.strIdx, bkrefNode, bkrefStr, opOpenSubexp) {
		return false
	}
	m.addEntry(bkrefNode, bkrefStr, top.strIdx, last.strIdx)
	m.cleanStateLogIfNeeded(bkrefStr + last.strIdx - top.strIdx)
	return true
}

// findSubexpNode is find_subexp_node.
func (d *dfa) findSubexpNode(nodes nodeSet, subexpIdx int, typ tokType) int {
	for _, n := range nodes {
		if d.nodes[n].typ == typ && d.nodes[n].idx == subexpIdx {
			return n
		}
	}
	return -1
}

// grow makes room in a path for index i.
func (p *stateArray) grow(i int) {
	if i >= len(p.array) {
		p.array = append(p.array, make([]*state, i+1-len(p.array))...)
	}
}

// checkArrival is check_arrival: can topNode at topStr reach lastNode at
// lastStr? The path keeps its states for later calls.
func (m *matcher) checkArrival(path *stateArray, topNode, topStr, lastNode, lastStr int, typ tokType) bool {
	d, c, in := m.d, m.c, m.in
	subexpNum := d.nodes[topNode].idx
	path.grow(lastStr + m.maxMBElem)
	strIdx := topStr
	if path.nextIdx != 0 {
		strIdx = path.nextIdx
	}
	// glibc points mctx->state_log at the path; here it is passed down
	backupCur := in.cur
	log := path
	in.cur = strIdx

	var cur *state
	var next nodeSet
	ctx := in.contextAt(strIdx - 1)
	if strIdx == topStr {
		next = d.checkArrivalExpandEcl(newSet1(topNode), subexpNum, typ)
	} else {
		cur = log.array[strIdx]
		if cur != nil && cur.hasBackref {
			next = cur.nodes.clone()
		}
	}
	if strIdx == topStr || (cur != nil && cur.hasBackref) {
		if len(next) > 0 {
			next = m.expandBkrefCache(log, next, strIdx, subexpNum, typ)
		}
		cur = c.acquireContext(next, ctx)
		log.array[strIdx] = cur
	}
	for nullCnt := 0; strIdx < lastStr && nullCnt <= m.maxMBElem; {
		next = nil
		log.grow(strIdx + 1)
		if s := log.array[strIdx+1]; s != nil {
			next.merge(s.nodes)
		}
		if cur != nil {
			next = m.checkArrivalAddNextNodes(log, strIdx, cur.nonEps, next)
		}
		strIdx++
		if len(next) > 0 {
			next = d.checkArrivalExpandEcl(next, subexpNum, typ)
			next = m.expandBkrefCache(log, next, strIdx, subexpNum, typ)
		}
		cur = c.acquireContext(next, in.contextAt(strIdx-1))
		log.array[strIdx] = cur
		if cur == nil {
			nullCnt++
		} else {
			nullCnt = 0
		}
	}
	var curNodes nodeSet
	reached := false
	if s := log.array[lastStr]; s != nil {
		curNodes = s.nodes
		reached = true
	}
	path.nextIdx = strIdx
	in.cur = backupCur
	return reached && curNodes.has(lastNode)
}

// checkArrivalAddNextNodes is check_arrival_add_next_nodes.
func (m *matcher) checkArrivalAddNextNodes(log *stateArray, strIdx int, cur, next nodeSet) nodeSet {
	d, c := m.d, m.c
	for _, n := range cur {
		k := 0
		if d.nodes[n].acceptMB {
			k = m.checkNodeAcceptBytes(n, strIdx)
			if k > 1 {
				nextIdx := strIdx + k
				log.grow(nextIdx)
				var u nodeSet
				if ds := log.array[nextIdx]; ds != nil {
					u.merge(ds.nodes)
				}
				u.insert(d.nexts[n])
				log.array[nextIdx] = c.acquire(u)
			}
		}
		if k != 0 || m.checkNodeAccept(n, strIdx) {
			next.insert(d.nexts[n])
		}
	}
	return next
}

// checkArrivalExpandEcl is check_arrival_expand_ecl: the epsilon closures
// of cur, cut at the subexpression boundary of type typ.
func (d *dfa) checkArrivalExpandEcl(cur nodeSet, exSubexp int, typ tokType) nodeSet {
	var out nodeSet
	for _, n := range cur {
		ecl := d.eclosures[n]
		if d.findSubexpNode(ecl, exSubexp, typ) == -1 {
			out.merge(ecl)
		} else {
			d.checkArrivalExpandEclSub(&out, n, exSubexp, typ)
		}
	}
	return out
}

// checkArrivalExpandEclSub is check_arrival_expand_ecl_sub.
func (d *dfa) checkArrivalExpandEclSub(dst *nodeSet, target, exSubexp int, typ tokType) {
	for n := target; !dst.has(n); {
		if d.nodes[n].typ == typ && d.nodes[n].idx == exSubexp {
			if typ == opCloseSubexp {
				dst.insert(n)
			}
			break
		}
		dst.insert(n)
		if len(d.edests[n]) == 0 {
			break
		}
		if len(d.edests[n]) == 2 {
			d.checkArrivalExpandEclSub(dst, d.edests[n][1], exSubexp, typ)
		}
		n = d.edests[n][0]
	}
}

// expandBkrefCache is expand_bkref_cache.
func (m *matcher) expandBkrefCache(log *stateArray, cur nodeSet, curStr, subexpNum int, typ tokType) nodeSet {
	d, c := m.d, m.c
	start := m.searchCurBkrefEntry(curStr)
	if start == -1 {
		return cur
	}
restart:
	for i := start; ; i++ {
		ent := m.bkrefEnts[i]
		if cur.has(ent.node) {
			toIdx := curStr + ent.subexpTo - ent.subexpFrom
			if toIdx == curStr {
				// an epsilon back-reference: re-check every node
				next := d.edests[ent.node][0]
				if !cur.has(next) {
					cur.merge(d.checkArrivalExpandEcl(newSet1(next), subexpNum, typ))
					goto restart
				}
			} else {
				next := d.nexts[ent.node]
				log.grow(toIdx)
				var u nodeSet
				if s := log.array[toIdx]; s != nil {
					if !s.nodes.has(next) {
						u = s.nodes.clone()
						u.insert(next)
						log.array[toIdx] = c.acquire(u)
					}
				} else {
					log.array[toIdx] = c.acquire(newSet1(next))
				}
			}
		}
		if !ent.more {
			break
		}
	}
	return cur
}

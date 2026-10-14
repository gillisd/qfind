package regex

// siftCtx is re_sift_context_t. Copies share the sifted and limited state
// arrays, as glibc's struct copies share the pointers.
type siftCtx struct {
	sifted     []*state
	limited    []*state
	lastNode   int
	lastStrIdx int
	limits     nodeSet
}

// pruneImpossibleNodes is prune_impossible_nodes for a pattern with
// back-references: it sifts the state log backwards from the halt node,
// retrying with shorter matches; false is REG_NOMATCH.
func (m *matcher) pruneImpossibleNodes() bool {
	matchLast := m.matchLast
	haltNode := m.lastNode
	sifted := make([]*state, matchLast+1)
	lim := make([]*state, matchLast+1)
	for {
		clear(lim)
		sctx := &siftCtx{sifted: sifted, limited: lim, lastNode: haltNode, lastStrIdx: matchLast}
		m.siftStatesBackward(sctx)
		if sifted[0] != nil || lim[0] != nil {
			break
		}
		for {
			matchLast--
			if matchLast < 0 {
				return false
			}
			if s := m.stateLog[matchLast]; s != nil && s.halt {
				if haltNode = m.checkHaltStateContext(s, matchLast); haltNode != 0 {
					break
				}
			}
		}
	}
	m.mergeStateArray(sifted, lim, matchLast+1)
	m.stateLog = sifted
	m.lastNode = haltNode
	m.matchLast = matchLast
	return true
}

// siftStatesBackward is sift_states_backward.
func (m *matcher) siftStatesBackward(sctx *siftCtx) {
	strIdx := sctx.lastStrIdx
	curDest := newSet1(sctx.lastNode)
	m.updateCurSiftedState(sctx, strIdx, &curDest)
	nullCnt := 0
	for strIdx > 0 {
		if sctx.sifted[strIdx] == nil {
			nullCnt++
		} else {
			nullCnt = 0
		}
		if nullCnt > m.maxMBElem {
			clear(sctx.sifted[:strIdx])
			return
		}
		curDest = nil
		strIdx--
		if m.stateLog[strIdx] != nil {
			curDest = m.buildSiftedStates(sctx, strIdx, curDest)
		}
		m.updateCurSiftedState(sctx, strIdx, &curDest)
	}
}

// buildSiftedStates is build_sifted_states.
func (m *matcher) buildSiftedStates(sctx *siftCtx, strIdx int, curDest nodeSet) nodeSet {
	d := m.d
	for _, prev := range m.stateLog[strIdx].nonEps {
		k := 0
		if d.nodes[prev].acceptMB {
			k = m.siftStatesIterMB(sctx, prev, strIdx, sctx.lastStrIdx)
		}
		if k == 0 && m.checkNodeAccept(prev, strIdx) && stateHas(sctx.sifted[strIdx+1], d.nexts[prev]) {
			k = 1
		}
		if k == 0 {
			continue
		}
		if len(sctx.limits) > 0 && m.checkDstLimits(sctx.limits, d.nexts[prev], strIdx+k, prev, strIdx) {
			continue
		}
		curDest.insert(prev)
	}
	return curDest
}

// stateHas is STATE_NODE_CONTAINS.
func stateHas(s *state, node int) bool { return s != nil && s.nodes.has(node) }

// mergeStateArray is merge_state_array.
func (m *matcher) mergeStateArray(dst, src []*state, num int) {
	for i := 0; i < num; i++ {
		switch {
		case dst[i] == nil:
			dst[i] = src[i]
		case src[i] != nil:
			dst[i] = m.c.acquire(union(dst[i].nodes, src[i].nodes))
		}
	}
}

// updateCurSiftedState is update_cur_sifted_state.
func (m *matcher) updateCurSiftedState(sctx *siftCtx, strIdx int, dest *nodeSet) {
	var candidates nodeSet
	hasCand := m.stateLog[strIdx] != nil
	if hasCand {
		candidates = m.stateLog[strIdx].nodes
	}
	if len(*dest) == 0 {
		sctx.sifted[strIdx] = nil
	} else {
		if hasCand {
			m.addEpsilonSrcNodes(dest, candidates)
			if len(sctx.limits) > 0 {
				m.checkSubexpLimits(dest, candidates, sctx.limits, strIdx)
			}
		}
		sctx.sifted[strIdx] = m.c.acquire(*dest)
	}
	if hasCand && m.stateLog[strIdx].hasBackref {
		m.siftStatesBkref(sctx, strIdx, candidates)
	}
}

// addEpsilonSrcNodes is add_epsilon_src_nodes: dest gains the candidates
// that reach it by epsilon transitions.
func (m *matcher) addEpsilonSrcNodes(dest *nodeSet, candidates nodeSet) {
	s := m.c.acquire(*dest)
	if !s.hasInvecl {
		s.hasInvecl = true
		for _, e := range *dest {
			s.inveclosure.merge(m.d.inveclosures[e])
		}
	}
	dest.addIntersect(candidates, s.inveclosure)
}

// subEpsilonSrcNodes is sub_epsilon_src_nodes.
func (m *matcher) subEpsilonSrcNodes(node int, dest *nodeSet, candidates nodeSet) {
	d := m.d
	inv := d.inveclosures[node]
	var except nodeSet
	for _, cur := range inv {
		if cur == node || !isEpsilon(d.nodes[cur].typ) {
			continue
		}
		e1, e2 := d.edests[cur][0], -1
		if len(d.edests[cur]) > 1 {
			e2 = d.edests[cur][1]
		}
		if (!inv.has(e1) && dest.has(e1)) || (e2 > 0 && !inv.has(e2) && dest.has(e2)) {
			except.addIntersect(candidates, d.inveclosures[cur])
		}
	}
	for _, cur := range inv {
		if !except.has(cur) {
			dest.removeAt(dest.contains(cur) - 1)
		}
	}
}

// checkDstLimits is check_dst_limits.
func (m *matcher) checkDstLimits(limits nodeSet, dstNode, dstIdx, srcNode, srcIdx int) bool {
	d := m.d
	dstBkref := m.searchCurBkrefEntry(dstIdx)
	srcBkref := m.searchCurBkrefEntry(srcIdx)
	for _, lim := range limits {
		subexpIdx := d.nodes[m.bkrefEnts[lim].node].idx
		dstPos := m.checkDstLimitsCalcPos(lim, subexpIdx, dstNode, dstIdx, dstBkref)
		srcPos := m.checkDstLimitsCalcPos(lim, subexpIdx, srcNode, srcIdx, srcBkref)
		if srcPos != dstPos {
			return true
		}
	}
	return false
}

// checkDstLimitsCalcPos1 is check_dst_limits_calc_pos_1.
func (m *matcher) checkDstLimitsCalcPos1(boundaries, subexpIdx, fromNode, bkrefIdx int) int {
	d := m.d
	for _, node := range d.eclosures[fromNode] {
		switch d.nodes[node].typ {
		case opBackRef:
			if bkrefIdx == -1 {
				break
			}
			for i := bkrefIdx; ; i++ {
				ent := &m.bkrefEnts[i]
				if ent.node == node && !(subexpIdx < 64 && ent.epsReachable&(1<<subexpIdx) == 0) {
					// recurse towards the OPEN/CLOSE_SUBEXP cases, but not
					// into the same node: ()\1*\1* would loop forever
					dst := d.edests[node][0]
					if dst == fromNode {
						if boundaries&1 != 0 {
							return -1
						}
						return 0
					}
					cpos := m.checkDstLimitsCalcPos1(boundaries, subexpIdx, dst, bkrefIdx)
					if cpos == -1 {
						return -1
					}
					if cpos == 0 && boundaries&2 != 0 {
						return 0
					}
					if subexpIdx < 64 {
						m.bkrefEnts[i].epsReachable &^= 1 << subexpIdx
					}
				}
				if !m.bkrefEnts[i].more {
					break
				}
			}
		case opOpenSubexp:
			if boundaries&1 != 0 && subexpIdx == d.nodes[node].idx {
				return -1
			}
		case opCloseSubexp:
			if boundaries&2 != 0 && subexpIdx == d.nodes[node].idx {
				return 0
			}
		}
	}
	if boundaries&2 != 0 {
		return 1
	}
	return 0
}

// checkDstLimitsCalcPos is check_dst_limits_calc_pos.
func (m *matcher) checkDstLimitsCalcPos(limit, subexpIdx, fromNode, strIdx, bkrefIdx int) int {
	lim := &m.bkrefEnts[limit]
	if strIdx < lim.subexpFrom {
		return -1
	}
	if lim.subexpTo < strIdx {
		return 1
	}
	boundaries := 0
	if strIdx == lim.subexpFrom {
		boundaries |= 1
	}
	if strIdx == lim.subexpTo {
		boundaries |= 2
	}
	if boundaries == 0 {
		return 0
	}
	return m.checkDstLimitsCalcPos1(boundaries, subexpIdx, fromNode, bkrefIdx)
}

// checkSubexpLimits is check_subexp_limits: drops the nodes of dest that
// break a back-reference limit.
func (m *matcher) checkSubexpLimits(dest *nodeSet, candidates, limits nodeSet, strIdx int) {
	d := m.d
	for _, lim := range limits {
		ent := &m.bkrefEnts[lim]
		if strIdx <= ent.subexpFrom || ent.strIdx < strIdx {
			continue
		}
		subexpIdx := d.nodes[ent.node].idx
		if ent.subexpTo == strIdx {
			ops, cls := -1, -1
			for _, node := range *dest {
				t := &d.nodes[node]
				if t.typ == opOpenSubexp && subexpIdx == t.idx {
					ops = node
				} else if t.typ == opCloseSubexp && subexpIdx == t.idx {
					cls = node
				}
			}
			if ops >= 0 {
				m.subEpsilonSrcNodes(ops, dest, candidates)
			}
			if cls >= 0 {
				for i := 0; i < len(*dest); i++ {
					node := (*dest)[i]
					if !d.inveclosures[node].has(cls) && !d.eclosures[node].has(cls) {
						m.subEpsilonSrcNodes(node, dest, candidates)
						i--
					}
				}
			}
		} else {
			for i := 0; i < len(*dest); i++ {
				node := (*dest)[i]
				t := &d.nodes[node]
				if (t.typ == opCloseSubexp || t.typ == opOpenSubexp) && subexpIdx == t.idx {
					m.subEpsilonSrcNodes(node, dest, candidates)
				}
			}
		}
	}
}

// siftStatesBkref is sift_states_bkref.
func (m *matcher) siftStatesBkref(sctx *siftCtx, strIdx int, candidates nodeSet) {
	d := m.d
	first := m.searchCurBkrefEntry(strIdx)
	if first == -1 {
		return
	}
	var local *siftCtx
	for _, node := range candidates {
		if node == sctx.lastNode && strIdx == sctx.lastStrIdx {
			continue // ()\1+ would loop forever
		}
		if d.nodes[node].typ != opBackRef {
			continue
		}
		for enabled := first; ; enabled++ {
			ent := m.bkrefEnts[enabled]
			if ent.node == node {
				subLen := ent.subexpTo - ent.subexpFrom
				toIdx := strIdx + subLen
				dst := d.edests[node][0]
				if subLen != 0 {
					dst = d.nexts[node]
				}
				if !(toIdx > sctx.lastStrIdx || sctx.sifted[toIdx] == nil ||
					!stateHas(sctx.sifted[toIdx], dst) ||
					m.checkDstLimits(sctx.limits, node, strIdx, dst, toIdx)) {
					if local == nil {
						cp := *sctx
						cp.limits = sctx.limits.clone()
						local = &cp
					}
					local.lastNode = node
					local.lastStrIdx = strIdx
					local.limits.insert(enabled)
					cur := local.sifted[strIdx]
					m.siftStatesBackward(local)
					if sctx.limited != nil {
						m.mergeStateArray(sctx.limited, local.sifted, strIdx+1)
					}
					local.sifted[strIdx] = cur
					local.limits.remove(enabled)
				}
			}
			if !m.bkrefEnts[enabled].more {
				break
			}
		}
	}
}

// siftStatesIterMB is sift_states_iter_mb.
func (m *matcher) siftStatesIterMB(sctx *siftCtx, node, strIdx, maxStrIdx int) int {
	k := m.checkNodeAcceptBytes(node, strIdx)
	if k > 0 && strIdx+k <= maxStrIdx && !stateHas(sctx.sifted[strIdx+k], m.d.nexts[node]) {
		k = 0
	}
	return k
}

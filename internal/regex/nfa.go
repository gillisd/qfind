package regex

// dfa is re_dfa_t without the state table, which lives in a per-search
// cache (see state.go): the NFA nodes and their links, built once by
// Compile and read-only afterwards.
type dfa struct {
	nodes        []token
	nexts        []int
	orgIndices   []int
	edests       []nodeSet
	eclosures    []nodeSet
	eclState     []int8 // 0: not computed, -1: in progress, 1: done
	inveclosures []nodeSet
	strTree      *tree
	sbChar       *bitset
	initNode     int
	initNodes    nodeSet // create_initial_state's node set
	nbackref     int
	nsub         int // re_nsub

	usedBkrefMap      uint64
	completedBkrefMap uint64

	hasPluralMatch bool
	hasMBNode      bool
	isUTF8         bool
	wordOpsUsed    bool
	mbCurMax       int
	wordChar       bitset
	syntax         Syntax
	subexpMap      []int
}

// initWordChar is init_word_char (map_notascii is 0 in C and C.UTF-8):
// ASCII alphanumerics and '_' are word bytes.
func (d *dfa) initWordChar() {
	d.wordOpsUsed = true
	for c := 0; c < 0x80; c++ {
		if isWordByte(byte(c)) {
			d.wordChar.set(c)
		}
	}
}

// addNode is re_dfa_add_node.
func (d *dfa) addNode(t token) int {
	t.constraint = 0
	t.acceptMB = (t.typ == opPeriod && d.mbCurMax > 1) || t.typ == complexBracket
	d.nodes = append(d.nodes, t)
	d.nexts = append(d.nexts, -1)
	d.orgIndices = append(d.orgIndices, 0)
	d.edests = append(d.edests, nil)
	d.eclosures = append(d.eclosures, nil)
	d.eclState = append(d.eclState, 0)
	return len(d.nodes) - 1
}

// analyze is regcomp.c analyze.
func (d *dfa) analyze(p *parser) errcode {
	d.subexpMap = make([]int, d.nsub)
	for i := range d.subexpMap {
		d.subexpMap[i] = i
	}
	preorder(d.strTree, d.optimizeSubexps)
	same := true
	for i, v := range d.subexpMap {
		same = same && v == i
	}
	if same {
		d.subexpMap = nil
	}
	if err := postorder(d.strTree, func(n *tree) errcode { return p.lowerSubexps(n) }); err != regNoError {
		return err
	}
	postorder(d.strTree, d.calcFirst)
	preorder(d.strTree, calcNext)
	preorder(d.strTree, d.linkNFANodes)
	d.calcEclosure()
	// regexec only uses inveclosures in prune_impossible_nodes, which for
	// find (nmatch == 1) runs only with back-references.
	if d.nbackref > 0 {
		d.calcInveclosure()
	}
	return regNoError
}

// optimizeSubexps is optimize_subexps: a SUBEXP directly containing
// another is merged into it.
func (d *dfa) optimizeSubexps(n *tree) errcode {
	if n.tok.typ == opBackRef && d.subexpMap != nil {
		idx := n.tok.idx
		n.tok.setIdx(d.subexpMap[idx])
		d.usedBkrefMap |= 1 << n.tok.idx
	} else if n.tok.typ == subexp && n.left != nil && n.left.tok.typ == subexp {
		other := n.left.tok.idx
		n.left = n.left.left
		if n.left != nil {
			n.left.parent = n
		}
		d.subexpMap[other] = d.subexpMap[n.tok.idx]
		if other < 64 {
			d.usedBkrefMap &^= 1 << other
		}
	}
	return regNoError
}

// lowerSubexps is lower_subexps (no_sub is 0 for re_compile_pattern).
func (p *parser) lowerSubexps(n *tree) errcode {
	if n.left != nil && n.left.tok.typ == subexp {
		n.left = p.lowerSubexp(n.left)
		if n.left != nil {
			n.left.parent = n
		}
	}
	if n.right != nil && n.right.tok.typ == subexp {
		n.right = p.lowerSubexp(n.right)
		if n.right != nil {
			n.right.parent = n
		}
	}
	return regNoError
}

// lowerSubexp is lower_subexp: SUBEXP becomes OPEN_SUBEXP body CLOSE_SUBEXP.
func (p *parser) lowerSubexp(n *tree) *tree {
	body := n.left
	op := p.createTree(nil, nil, opOpenSubexp)
	cls := p.createTree(nil, nil, opCloseSubexp)
	tree1 := cls
	if body != nil {
		tree1 = p.createTree(body, cls, concat)
	}
	t := p.createTree(op, tree1, concat)
	op.tok.setIdx(n.tok.idx)
	cls.tok.setIdx(n.tok.idx)
	op.tok.optSubexp, cls.tok.optSubexp = n.tok.optSubexp, n.tok.optSubexp
	return t
}

// calcFirst is calc_first: creates the automaton nodes, in postorder.
func (d *dfa) calcFirst(n *tree) errcode {
	if n.tok.typ == concat {
		n.first = n.left.first
		n.nodeIdx = n.left.nodeIdx
		return regNoError
	}
	n.first = n
	n.nodeIdx = d.addNode(n.tok)
	if n.tok.typ == anchor {
		d.nodes[n.nodeIdx].constraint = n.tok.ctx
	}
	return regNoError
}

// calcNext is calc_next, in preorder.
func calcNext(n *tree) errcode {
	switch n.tok.typ {
	case opDupAsterisk:
		n.left.next = n
	case concat:
		n.left.next = n.right.first
		n.right.next = n.next
	default:
		if n.left != nil {
			n.left.next = n.next
		}
		if n.right != nil {
			n.right.next = n.next
		}
	}
	return regNoError
}

// linkNFANodes is link_nfa_nodes.
func (d *dfa) linkNFANodes(n *tree) errcode {
	idx := n.nodeIdx
	switch n.tok.typ {
	case concat, endOfRE:
	case opDupAsterisk, opAlt:
		d.hasPluralMatch = true
		var left, right int
		if n.left != nil {
			left = n.left.first.nodeIdx
		} else {
			left = n.next.nodeIdx
		}
		if n.right != nil {
			right = n.right.first.nodeIdx
		} else {
			right = n.next.nodeIdx
		}
		d.edests[idx] = newSet2(left, right)
	case anchor, opOpenSubexp, opCloseSubexp:
		d.edests[idx] = newSet1(n.next.nodeIdx)
	case opBackRef:
		d.nexts[idx] = n.next.nodeIdx
		d.edests[idx] = newSet1(d.nexts[idx])
	default:
		d.nexts[idx] = n.next.nodeIdx
	}
	return regNoError
}

// duplicateNodeClosure is duplicate_node_closure: clones the epsilon
// closure of rootNode so that the clones carry initConstraint.
func (d *dfa) duplicateNodeClosure(topOrg, topClone, rootNode, initConstraint int) {
	constraint := initConstraint
	org, clone := topOrg, topClone
	for {
		var orgDest, cloneDest int
		switch {
		case d.nodes[org].typ == opBackRef:
			orgDest = d.nexts[org]
			d.edests[clone] = nil
			cloneDest = d.duplicateNode(orgDest, constraint)
			d.nexts[clone] = d.nexts[org]
			d.edests[clone].insert(cloneDest)
		case len(d.edests[org]) == 0:
			d.nexts[clone] = d.nexts[org]
			return
		case len(d.edests[org]) == 1:
			orgDest = d.edests[org][0]
			d.edests[clone] = nil
			if org == rootNode && clone != org {
				// a loop in the closure: tie it to the root's destination
				d.edests[clone].insert(orgDest)
				return
			}
			constraint |= d.nodes[org].constraint
			cloneDest = d.duplicateNode(orgDest, constraint)
			d.edests[clone].insert(cloneDest)
		default: // two destinations: '|' and '*'
			orgDest = d.edests[org][0]
			second := d.edests[org][1] // glibc's emptied set still holds it
			d.edests[clone] = nil
			cloneDest = d.searchDuplicatedNode(orgDest, constraint)
			if cloneDest == -1 {
				cloneDest = d.duplicateNode(orgDest, constraint)
				d.edests[clone].insert(cloneDest)
				d.duplicateNodeClosure(orgDest, cloneDest, rootNode, constraint)
			} else {
				d.edests[clone].insert(cloneDest)
			}
			orgDest = second
			cloneDest = d.duplicateNode(orgDest, constraint)
			d.edests[clone].insert(cloneDest)
		}
		org, clone = orgDest, cloneDest
	}
}

// searchDuplicatedNode is search_duplicated_node. Like glibc it scans back
// over every node marked duplicated, including the copies duplicate_tree
// made, whose org_indices glibc never sets (zero here).
func (d *dfa) searchDuplicatedNode(org, constraint int) int {
	for idx := len(d.nodes) - 1; d.nodes[idx].duplicated && idx > 0; idx-- {
		if org == d.orgIndices[idx] && constraint == d.nodes[idx].constraint {
			return idx
		}
	}
	return -1
}

// duplicateNode is duplicate_node.
func (d *dfa) duplicateNode(org, constraint int) int {
	dup := d.addNode(d.nodes[org])
	d.nodes[dup].constraint = constraint | d.nodes[org].constraint
	d.nodes[dup].duplicated = true
	d.orgIndices[dup] = org
	return dup
}

// calcInveclosure is calc_inveclosure.
func (d *dfa) calcInveclosure() {
	d.inveclosures = make([]nodeSet, len(d.nodes))
	for src := range d.nodes {
		for _, e := range d.eclosures[src] {
			d.inveclosures[e].insertLast(src)
		}
	}
}

// calcEclosure is calc_eclosure.
func (d *dfa) calcEclosure() {
	incomplete := false
	for idx := 0; ; idx++ {
		if idx == len(d.nodes) {
			if !incomplete {
				break
			}
			incomplete = false
			idx = 0
		}
		if d.eclState[idx] != 0 {
			continue
		}
		d.calcEclosureIter(idx, true)
		if d.eclState[idx] == 0 {
			incomplete = true
		}
	}
}

// calcEclosureIter is calc_eclosure_iter. A constrained node duplicates
// its closure so the constraint reaches the nodes after it, unless its
// destination is already a duplicate: in glibc that also skips the
// copies duplicate_tree made, so an anchor inside an interval copy, as
// in \(\<a\)\{2\}, constrains only the first copy.
func (d *dfa) calcEclosureIter(node int, root bool) nodeSet {
	ecl := nodeSet{node}
	incomplete := false
	d.eclState[node] = -1
	if d.nodes[node].constraint != 0 && len(d.edests[node]) > 0 && !d.nodes[d.edests[node][0]].duplicated {
		d.duplicateNodeClosure(node, node, node, d.nodes[node].constraint)
	}
	if isEpsilon(d.nodes[node].typ) {
		for i := 0; i < len(d.edests[node]); i++ {
			edest := d.edests[node][i]
			if d.eclState[edest] == -1 {
				incomplete = true
				continue
			}
			var elem nodeSet
			if d.eclState[edest] == 0 {
				elem = d.calcEclosureIter(edest, false)
			} else {
				elem = d.eclosures[edest]
			}
			ecl.merge(elem)
			if d.eclState[edest] == 0 {
				incomplete = true
			}
		}
	}
	if incomplete && !root {
		d.eclState[node] = 0
	} else {
		d.eclosures[node] = ecl
		d.eclState[node] = 1
	}
	return ecl
}

// optimizeUTF8 is optimize_utf8: a pattern without word anchors or
// COMPLEX_BRACKETs is searched byte by byte, '.' becoming OP_UTF8_PERIOD.
func (d *dfa) optimizeUTF8() {
	mbChars, hasPeriod := false, false
	for i := range d.nodes {
		n := &d.nodes[i]
		switch n.typ {
		case character:
			if n.c >= 0x80 {
				mbChars = true
			}
		case anchor:
			switch n.ctx {
			case lineFirst, lineLast, bufFirst, bufLast:
			default:
				return
			}
		case opPeriod:
			hasPeriod = true
		case complexBracket:
			return
		case simpleBracket:
			for c := 0x80; c < 0x100; c++ {
				if n.sbcset.contains(c) {
					return
				}
			}
		}
	}
	if mbChars || hasPeriod {
		for i := range d.nodes {
			n := &d.nodes[i]
			if n.typ == character && n.c >= 0x80 {
				n.mbPartial = false
			} else if n.typ == opPeriod {
				n.typ = opUTF8Period
			}
		}
	}
	d.mbCurMax = 1
	d.isUTF8 = false
	d.hasMBNode = d.nbackref > 0 || hasPeriod
}

// createInitialNodes is the node-set half of create_initial_state; the
// states themselves are acquired per search cache (state.go).
func (d *dfa) createInitialNodes() {
	first := d.strTree.first.nodeIdx
	d.initNode = first
	init := d.eclosures[first].clone()
	if d.nbackref > 0 {
		for i := 0; i < len(init); i++ {
			nodeIdx := init[i]
			if d.nodes[nodeIdx].typ != opBackRef {
				continue
			}
			found := false
			for _, e := range init {
				cl := &d.nodes[e]
				if cl.typ == opCloseSubexp && cl.idx == d.nodes[nodeIdx].idx {
					found = true
					break
				}
			}
			if !found {
				continue
			}
			dest := d.edests[nodeIdx][0]
			if !init.has(dest) {
				init.merge(d.eclosures[dest])
				i = 0
			}
		}
	}
	d.initNodes = init
}

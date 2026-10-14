package regex

// tree is bin_tree_t.
type tree struct {
	parent, left, right *tree
	first, next         *tree
	tok                 token
	nodeIdx             int
}

// maxTrees bounds the parse tree that interval expansion may build. glibc
// allocates until malloc fails; past this size we report the same
// REG_ESPACE ("Memory exhausted") instead of exhausting memory first.
const maxTrees = 1 << 24

// parser holds the state of regcomp.c's parse_* functions.
type parser struct {
	in  *reString
	dfa *dfa
	err errcode
	n   int // trees created
}

// createTokenTree is create_token_tree.
func (p *parser) createTokenTree(left, right *tree, t *token) *tree {
	p.n++
	nt := &tree{left: left, right: right, tok: *t, nodeIdx: -1}
	nt.tok.duplicated, nt.tok.optSubexp = false, false
	if left != nil {
		left.parent = nt
	}
	if right != nil {
		right.parent = nt
	}
	return nt
}

// createTree is create_tree.
func (p *parser) createTree(left, right *tree, typ tokType) *tree {
	return p.createTokenTree(left, right, &token{typ: typ})
}

// postorder is regcomp.c postorder, which visits in glibc's exact order.
func postorder(root *tree, fn func(*tree) errcode) errcode {
	node := root
	for {
		for node.left != nil || node.right != nil {
			if node.left != nil {
				node = node.left
			} else {
				node = node.right
			}
		}
		var prev *tree
		for {
			if err := fn(node); err != regNoError {
				return err
			}
			if node.parent == nil {
				return regNoError
			}
			prev = node
			node = node.parent
			if !(node.right == prev || node.right == nil) {
				break
			}
		}
		node = node.right
	}
}

// preorder is regcomp.c preorder.
func preorder(root *tree, fn func(*tree) errcode) errcode {
	node := root
	for {
		if err := fn(node); err != regNoError {
			return err
		}
		if node.left != nil {
			node = node.left
			continue
		}
		var prev *tree
		for node.right == prev || node.right == nil {
			prev = node
			node = node.parent
			if node == nil {
				return regNoError
			}
		}
		node = node.right
	}
}

// duplicateTree is duplicate_tree: the copies are marked duplicated.
func (p *parser) duplicateTree(root *tree) *tree {
	var dupRoot *tree
	pNew := &dupRoot
	dupNode := root.parent
	node := root
	for {
		*pNew = p.createTokenTree(nil, nil, &node.tok)
		(*pNew).parent = dupNode
		(*pNew).tok.duplicated = true
		dupNode = *pNew
		if node.left != nil {
			node = node.left
			pNew = &dupNode.left
			continue
		}
		var prev *tree
		for node.right == prev || node.right == nil {
			prev = node
			node = node.parent
			dupNode = dupNode.parent
			if node == nil {
				return dupRoot
			}
		}
		node = node.right
		pNew = &dupNode.right
	}
}

// parse is regcomp.c parse: <reg_exp> CONCAT END_OF_RE.
func (p *parser) parse(syn Syntax) *tree {
	var cur token
	p.dfa.syntax = syn
	p.fetchToken(&cur, syn|CaretAnchorsHere)
	t := p.parseRegExp(&cur, syn, 0)
	if p.err != regNoError && t == nil {
		return nil
	}
	eor := p.createTree(nil, nil, endOfRE)
	if t != nil {
		return p.createTree(t, eor, concat)
	}
	return eor
}

// parseRegExp is parse_reg_exp: branch ('|' branch)*.
func (p *parser) parseRegExp(t *token, syn Syntax, nest int) *tree {
	d := p.dfa
	initial := d.completedBkrefMap
	tr := p.parseBranch(t, syn, nest)
	if p.err != regNoError && tr == nil {
		return nil
	}
	for t.typ == opAlt {
		p.fetchToken(t, syn|CaretAnchorsHere)
		var branch *tree
		if t.typ != opAlt && t.typ != endOfRE && (nest == 0 || t.typ != opCloseSubexp) {
			accumulated := d.completedBkrefMap
			d.completedBkrefMap = initial
			branch = p.parseBranch(t, syn, nest)
			if p.err != regNoError && branch == nil {
				return nil
			}
			d.completedBkrefMap |= accumulated
		}
		tr = p.createTree(tr, branch, opAlt)
	}
	return tr
}

// parseBranch is parse_branch: a concatenation of expressions.
func (p *parser) parseBranch(t *token, syn Syntax, nest int) *tree {
	tr := p.parseExpression(t, syn, nest)
	if p.err != regNoError && tr == nil {
		return nil
	}
	for t.typ != opAlt && t.typ != endOfRE && (nest == 0 || t.typ != opCloseSubexp) {
		expr := p.parseExpression(t, syn, nest)
		if p.err != regNoError && expr == nil {
			return nil
		}
		if tr != nil && expr != nil {
			tr = p.createTree(tr, expr, concat)
		} else if tr == nil {
			tr = expr
		}
	}
	return tr
}

// parseExpression is parse_expression.
func (p *parser) parseExpression(t *token, syn Syntax, nest int) *tree {
	d, in := p.dfa, p.in
	var tr *tree
	switch t.typ {
	case character:
		tr = p.createTokenTree(nil, nil, t)
		if d.mbCurMax > 1 {
			for !in.eoi() && !in.firstByte(in.cur) {
				p.fetchToken(t, syn)
				tr = p.createTree(tr, p.createTokenTree(nil, nil, t), concat)
			}
		}
	case opOpenSubexp:
		tr = p.parseSubExp(t, syn, nest+1)
		if p.err != regNoError && tr == nil {
			return nil
		}
	case opOpenBracket:
		tr = p.parseBracketExp(t, syn)
		if p.err != regNoError && tr == nil {
			return nil
		}
	case opBackRef:
		if d.completedBkrefMap&(1<<t.idx) == 0 {
			p.err = regESubreg
			return nil
		}
		d.usedBkrefMap |= 1 << t.idx
		tr = p.createTokenTree(nil, nil, t)
		d.nbackref++
		d.hasMBNode = true
	case opOpenDupNum, opDupAsterisk, opDupPlus, opDupQuestion:
		if t.typ == opOpenDupNum && syn&ContextInvalidDup != 0 {
			p.err = regBadRpt
			return nil
		}
		if syn&ContextInvalidOps != 0 {
			p.err = regBadRpt
			return nil
		}
		if syn&ContextIndepOps != 0 {
			p.fetchToken(t, syn)
			return p.parseExpression(t, syn, nest)
		}
		t.typ = character
		tr = p.createTokenTree(nil, nil, t)
	case opCloseSubexp:
		if syn&UnmatchedRightParenOrd == 0 {
			p.err = regERParen
			return nil
		}
		t.typ = character
		tr = p.createTokenTree(nil, nil, t)
	case opCloseDupNum:
		t.typ = character
		tr = p.createTokenTree(nil, nil, t)
	case anchor:
		if t.ctx&(wordDelim|notWordDelim|wordFirst|wordLast) != 0 && !d.wordOpsUsed {
			d.initWordChar()
		}
		if t.ctx == wordDelim || t.ctx == notWordDelim {
			var first *tree
			if t.ctx == wordDelim {
				t.setCtx(wordFirst)
				first = p.createTokenTree(nil, nil, t)
				t.setCtx(wordLast)
			} else {
				t.setCtx(insideWord)
				first = p.createTokenTree(nil, nil, t)
				t.setCtx(insideNotWord)
			}
			tr = p.createTree(first, p.createTokenTree(nil, nil, t), opAlt)
		} else {
			tr = p.createTokenTree(nil, nil, t)
		}
		// "^*" is an anchor and a literal '*': anchors are never repeated.
		p.fetchToken(t, syn)
		return tr
	case opPeriod:
		tr = p.createTokenTree(nil, nil, t)
		if d.mbCurMax > 1 {
			d.hasMBNode = true
		}
	case opWord, opNotWord:
		tr = p.buildCharclassOp("alnum", "_", t.typ == opNotWord)
	case opSpace, opNotSpace:
		tr = p.buildCharclassOp("space", "", t.typ == opNotSpace)
	case opAlt, endOfRE:
		return nil
	case backSlash:
		p.err = regEEscape
		return nil
	}
	p.fetchToken(t, syn)
	for t.typ == opDupAsterisk || t.typ == opDupPlus || t.typ == opDupQuestion || t.typ == opOpenDupNum {
		dup := p.parseDupOp(tr, t, syn)
		if p.err != regNoError && dup == nil {
			return nil
		}
		tr = dup
		if syn&ContextInvalidDup != 0 && (t.typ == opDupAsterisk || t.typ == opOpenDupNum) {
			p.err = regBadRpt // in a BRE, "a**" is an error
			return nil
		}
	}
	return tr
}

// parseSubExp is parse_sub_exp.
func (p *parser) parseSubExp(t *token, syn Syntax, nest int) *tree {
	d := p.dfa
	cur := d.nsub
	d.nsub++
	p.fetchToken(t, syn|CaretAnchorsHere)
	var tr *tree
	if t.typ != opCloseSubexp {
		tr = p.parseRegExp(t, syn, nest)
		if p.err == regNoError && t.typ != opCloseSubexp {
			p.err = regEParen
		}
		if p.err != regNoError {
			return nil
		}
	}
	if cur <= '9'-'1' {
		d.completedBkrefMap |= 1 << cur
	}
	tr = p.createTree(tr, nil, subexp)
	tr.tok.idx = cur
	return tr
}

// parseDupOp is parse_dup_op: <re>{n,m} becomes n copies of <re> followed
// by (<re>(<re>...)?)?, or by <re>* when m is unbounded.
func (p *parser) parseDupOp(elem *tree, t *token, syn Syntax) *tree {
	in := p.in
	startIdx, startTok := in.cur, *t
	var start, end int
	if t.typ == opOpenDupNum {
		start = p.fetchNumber(t, syn)
		if start == -1 {
			if t.typ == character && t.c == ',' {
				start = 0 // {,m} is {0,m}
			} else {
				p.err = regBadBR // <re>{} is invalid
				return nil
			}
		}
		if start != -2 {
			switch {
			case t.typ == opCloseDupNum:
				end = start
			case t.typ == character && t.c == ',':
				end = p.fetchNumber(t, syn)
			default:
				end = -2
			}
		}
		if start == -2 || end == -2 {
			if syn&InvalidIntervalOrd == 0 {
				if t.typ == endOfRE {
					p.err = regEBrace
				} else {
					p.err = regBadBR
				}
				return nil
			}
			// roll back: the '{' is an ordinary character
			in.cur = startIdx
			*t = startTok
			t.typ = character
			return elem
		}
		if (end != -1 && start > end) || t.typ != opCloseDupNum {
			p.err = regBadBR
			return nil
		}
		lim := end
		if end == -1 {
			lim = start
		}
		if lim > dupMax {
			p.err = regESize
			return nil
		}
	} else {
		start, end = 0, -1
		if t.typ == opDupPlus {
			start = 1
		}
		if t.typ == opDupQuestion {
			end = 1
		}
	}
	p.fetchToken(t, syn)
	if elem == nil {
		return nil
	}
	if start == 0 && end == 0 {
		return nil
	}
	if p.n+treeSize(elem)*(max(start, end)+1) > maxTrees {
		p.err = regESpace
		return nil
	}
	var tr, old *tree
	if start > 0 {
		tr = elem
		for i := 2; i <= start; i++ {
			elem = p.duplicateTree(elem)
			tr = p.createTree(tr, elem, concat)
		}
		if start == end {
			return tr
		}
		elem = p.duplicateTree(elem) // before it is marked optional
		old = tr
	}
	if elem.tok.typ == subexp {
		idx := elem.tok.idx
		postorder(elem, func(n *tree) errcode {
			if n.tok.typ == subexp && n.tok.idx == idx {
				n.tok.optSubexp = true
			}
			return regNoError
		})
	}
	typ := opAlt
	if end == -1 {
		typ = opDupAsterisk
	}
	tr = p.createTree(elem, nil, typ)
	for i := start + 2; i <= end; i++ {
		elem = p.duplicateTree(elem)
		tr = p.createTree(tr, elem, concat)
		tr = p.createTree(tr, nil, opAlt)
	}
	if old != nil {
		tr = p.createTree(old, tr, concat)
	}
	return tr
}

func treeSize(t *tree) int {
	if t == nil {
		return 0
	}
	return 1 + treeSize(t.left) + treeSize(t.right)
}

// fetchNumber is fetch_number: -1 for an empty field, -2 for an error,
// RE_DUP_MAX+1 when too large.
func (p *parser) fetchNumber(t *token, syn Syntax) int {
	num := -1
	for {
		p.fetchToken(t, syn)
		c := t.c
		if t.typ == endOfRE {
			return -2
		}
		if t.typ == opCloseDupNum || c == ',' {
			return num
		}
		switch {
		case t.typ != character || c < '0' || c > '9' || num == -2:
			num = -2
		case num == -1:
			num = int(c - '0')
		default:
			num = min(dupMax+1, num*10+int(c-'0'))
		}
	}
}

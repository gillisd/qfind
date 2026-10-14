package regex

// state is re_dfastate_t.
type state struct {
	hash          int
	nodes         nodeSet
	nonEps        nodeSet
	inveclosure   nodeSet
	hasInvecl     bool
	entrance      nodeSet
	trtable       *[256]*state
	wordTrtable   *[512]*state
	context       int
	halt          bool
	acceptMB      bool
	hasBackref    bool
	hasConstraint bool
}

// cache is the state table glibc keeps in re_dfa_t, plus the initial
// states create_initial_state puts there first. Each search borrows one
// from the Regexp's pool, so a compiled pattern stays safe for concurrent
// use; glibc instead shares one table under a lock.
type cache struct {
	d                                       *dfa
	table                                   map[int][]*state
	initState, initWord, initNL, initBegbuf *state
}

func newCache(d *dfa) *cache {
	c := &cache{d: d, table: map[int][]*state{}}
	c.initState = c.acquireContext(d.initNodes, 0)
	if c.initState.hasConstraint {
		c.initWord = c.acquireContext(d.initNodes, contextWord)
		c.initNL = c.acquireContext(d.initNodes, contextNewline)
		c.initBegbuf = c.acquireContext(d.initNodes, contextNewline|contextBegbuf)
	} else {
		c.initWord, c.initNL, c.initBegbuf = c.initState, c.initState, c.initState
	}
	return c
}

// stateHash is calc_state_hash.
func stateHash(nodes nodeSet, context int) int {
	h := len(nodes) + context
	for _, n := range nodes {
		h += n
	}
	return h
}

// acquire is re_acquire_state: a state whose (sifted) node set is nodes,
// whatever its context.
func (c *cache) acquire(nodes nodeSet) *state {
	if len(nodes) == 0 {
		return nil
	}
	h := stateHash(nodes, 0)
	for _, s := range c.table[h] {
		if s.nodes.equal(nodes) {
			return s
		}
	}
	return c.newCIState(nodes, h)
}

// acquireContext is re_acquire_state_context.
func (c *cache) acquireContext(nodes nodeSet, context int) *state {
	if len(nodes) == 0 {
		return nil
	}
	h := stateHash(nodes, context)
	for _, s := range c.table[h] {
		if s.context == context && s.entrance.equal(nodes) {
			return s
		}
	}
	return c.newCDState(nodes, context, h)
}

// register is register_state.
func (c *cache) register(s *state, h int) {
	s.hash = h
	for _, e := range s.nodes {
		if !isEpsilon(c.d.nodes[e].typ) {
			s.nonEps.insertLast(e)
		}
	}
	c.table[h] = append(c.table[h], s)
}

// newCIState is create_ci_newstate.
func (c *cache) newCIState(nodes nodeSet, h int) *state {
	s := &state{nodes: nodes.clone()}
	s.entrance = s.nodes
	for _, e := range nodes {
		n := &c.d.nodes[e]
		if n.typ == character && n.constraint == 0 {
			continue
		}
		s.acceptMB = s.acceptMB || n.acceptMB
		switch {
		case n.typ == endOfRE:
			s.halt = true
		case n.typ == opBackRef:
			s.hasBackref = true
		case n.typ == anchor || n.constraint != 0:
			s.hasConstraint = true
		}
	}
	c.register(s, h)
	return s
}

// newCDState is create_cd_newstate: nodes whose PREV constraint the
// context fails are dropped, but stay in the entrance nodes.
func (c *cache) newCDState(nodes nodeSet, context, h int) *state {
	s := &state{nodes: nodes.clone(), context: context}
	s.entrance = s.nodes
	separate := false
	removed := 0
	for i, e := range nodes {
		n := &c.d.nodes[e]
		if n.typ == character && n.constraint == 0 {
			continue
		}
		s.acceptMB = s.acceptMB || n.acceptMB
		if n.typ == endOfRE {
			s.halt = true
		} else if n.typ == opBackRef {
			s.hasBackref = true
		}
		if n.constraint != 0 {
			if !separate {
				s.entrance = nodes.clone()
				separate = true
				removed = 0
				s.hasConstraint = true
			}
			if notSatisfyPrev(n.constraint, context) {
				s.nodes.removeAt(i - removed)
				removed++
			}
		}
	}
	c.register(s, h)
	return s
}

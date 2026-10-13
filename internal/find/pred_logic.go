package find

// predAnd is pred.c pred_and.
func predAnd(r *Run, e *Entry, p *Predicate) bool {
	if p.left == nil || r.apply(e, p.left) {
		return r.apply(e, p.right)
	}
	return false
}

// predOr is pred.c pred_or.
func predOr(r *Run, e *Entry, p *Predicate) bool {
	if p.left == nil || !r.apply(e, p.left) {
		return r.apply(e, p.right)
	}
	return true
}

// predComma is pred.c pred_comma: both sides run, the right one decides.
func predComma(r *Run, e *Entry, p *Predicate) bool {
	if p.left != nil {
		r.apply(e, p.left)
	}
	return r.apply(e, p.right)
}

// predNegate is pred.c pred_negate.
func predNegate(r *Run, e *Entry, p *Predicate) bool { return !r.apply(e, p.right) }

// predOpenparen, predCloseparen, predTrue and predFalse are their pred.c
// namesakes.
func predOpenparen(*Run, *Entry, *Predicate) bool  { return true }
func predCloseparen(*Run, *Entry, *Predicate) bool { return true }
func predTrue(*Run, *Entry, *Predicate) bool       { return true }
func predFalse(*Run, *Entry, *Predicate) bool      { return false }

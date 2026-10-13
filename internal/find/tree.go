package find

// getExpr is tree.c get_expr: the tree for the expression at *input up to
// the next operator binding no tighter than prevPrec.
func (r *Run) getExpr(input **Predicate, prevPrec Prec, prevPred *Predicate) *Predicate {
	var next *Predicate
	this := *input
	if this == nil {
		r.fatalf(nil, "invalid expression")
	}
	switch this.typ {
	case NoType:
		r.fatalf(nil, "invalid expression")
	case BiOp:
		r.fatalf(nil, "invalid expression; you have used a binary operator '%s' with nothing before it.", this.name)
	case CloseParen:
		if prevPred == nil {
			r.fatalf(nil, "invalid expression: expected expression before closing parentheses '%s'.", this.name)
		}
		if (prevPred.typ == UniOp || prevPred.typ == BiOp) && !this.artificial {
			r.fatalf(nil, "expected an expression between '%s' and ')'", prevPred.name)
		} else if this.artificial {
			r.fatalf(nil, "expected an expression after '%s'", prevPred.name)
		}
		r.fatalf(nil, "invalid expression; you have too many ')'")
	case PrimaryType:
		next = this
		*input = this.next
	case UniOp:
		next = this
		*input = this.next
		next.right = r.getExpr(input, NegatePrec, next)
	case OpenParen:
		if this.next == nil || this.next.artificial {
			r.fatalf(nil, "invalid expression; expected to find a ')' but didn't see one. Perhaps you need an extra predicate after '%s'", this.name)
		}
		prevPred = this
		*input = this.next
		if (*input).typ == CloseParen {
			if prevPred.artificial {
				r.fatalf(nil, "invalid expression: expected expression before closing parentheses '%s'.", (*input).name)
			}
			r.fatalf(nil, "invalid expression; empty parentheses are not allowed.")
		}
		next = r.getExpr(input, NoPrec, prevPred)
		if *input == nil || (*input).typ != CloseParen {
			r.fatalf(nil, "invalid expression; I was expecting to find a ')' somewhere but did not see one.")
		}
		*input = (*input).next
	default:
		r.fatalf(nil, "oops -- invalid expression type! This is probably a bug.")
	}
	if *input == nil {
		return next
	}
	if (*input).prec > prevPrec {
		next = r.scanRest(input, next, prevPrec)
		if next == nil {
			r.fatalf(nil, "invalid expression")
		}
	}
	return next
}

// scanRest is tree.c scan_rest.
func (r *Run) scanRest(input **Predicate, head *Predicate, prevPrec Prec) *Predicate {
	if *input == nil || (*input).typ == CloseParen {
		return nil
	}
	tree := head
	for *input != nil && (*input).prec > prevPrec {
		switch (*input).typ {
		case NoType, PrimaryType, UniOp, OpenParen:
			r.fatalf(nil, "invalid expression")
		case BiOp:
			prev := *input
			(*input).left = tree
			tree = *input
			*input = (*input).next
			tree.right = r.getExpr(input, tree.prec, prev)
		case CloseParen:
			return tree
		default:
			r.fatalf(nil, "oops -- invalid expression type (%d)!  This is probably a bug.", (*input).typ)
		}
	}
	return tree
}

// setNewParent is tree.c set_new_parent.
func setNewParent(curr *Predicate, high Prec, prevp **Predicate) *Predicate {
	np := &Predicate{typ: BiOp, prec: high, cost: NeedsNothing}
	switch high {
	case CommaPrec:
		np.fn, np.id, np.name, np.estSuccessRate = predComma, fnComma, ",", 1.0
	case OrPrec:
		np.fn, np.id, np.name, np.estSuccessRate = predOr, fnOr, "-o", constrainRate(curr.estSuccessRate)
	case AndPrec:
		np.fn, np.id, np.name, np.estSuccessRate = predAnd, fnAnd, "-a", constrainRate(curr.estSuccessRate)
	}
	np.right = curr
	*prevp = np
	return np
}

// mergePred is tree.c merge_pred.
func mergePred(beg, end *Predicate, lastp **Predicate) {
	end.left = *lastp
	*lastp = beg
}

// constrainRate is tree.c constrain_rate.
func constrainRate(rate float32) float32 {
	switch {
	case rate > 1:
		return 1
	case rate < 0:
		return 0
	}
	return rate
}

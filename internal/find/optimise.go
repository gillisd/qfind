package find

// costLookup is tree.c costlookup[].
var costLookup = map[FuncID]Cost{
	fnAmin: NeedsStatInfo, fnAnd: NeedsNothing, fnAnewer: NeedsStatInfo, fnAtime: NeedsStatInfo,
	fnCloseParen: NeedsNothing, fnCmin: NeedsStatInfo, fnCnewer: NeedsStatInfo, fnComma: NeedsNothing,
	fnContext: NeedsAccessInfo, fnCtime: NeedsStatInfo, fnDelete: NeedsSyncDiskHit, fnEmpty: NeedsStatInfo,
	fnExec: NeedsEventualExec, fnExecdir: NeedsEventualExec, fnExecutable: NeedsAccessInfo, fnFalse: NeedsNothing,
	fnFprint: NeedsNothing, fnFprint0: NeedsNothing, fnFprintf: NeedsNothing, fnFstype: NeedsStatInfo,
	fnGid: NeedsStatInfo, fnGroup: NeedsStatInfo, fnIlname: NeedsLinkName, fnIname: NeedsNothing,
	fnInum: NeedsInodeNumber, fnIpath: NeedsNothing, fnLinks: NeedsStatInfo, fnLname: NeedsLinkName,
	fnLs: NeedsStatInfo, fnFls: NeedsStatInfo, fnMmin: NeedsStatInfo, fnMtime: NeedsStatInfo,
	fnName: NeedsNothing, fnNegate: NeedsNothing, fnNewer: NeedsStatInfo, fnNewerXY: NeedsStatInfo,
	fnNogroup: NeedsStatInfo, fnNouser: NeedsStatInfo, fnOk: NeedsUserInteraction, fnOkdir: NeedsUserInteraction,
	fnOpenParen: NeedsNothing, fnOr: NeedsNothing, fnPath: NeedsNothing, fnPerm: NeedsStatInfo,
	fnPrint: NeedsNothing, fnPrint0: NeedsNothing, fnPrune: NeedsNothing, fnQuit: NeedsNothing,
	fnReadable: NeedsAccessInfo, fnRegex: NeedsNothing, fnSamefile: NeedsStatInfo, fnSize: NeedsStatInfo,
	fnTrue: NeedsNothing, fnType: NeedsType, fnUid: NeedsStatInfo, fnUsed: NeedsStatInfo,
	fnUser: NeedsStatInfo, fnWritable: NeedsAccessInfo, fnXtype: NeedsType,
}

// getPredCost is tree.c get_pred_cost.
func getPredCost(p *Predicate) Cost {
	data := NeedsNothing
	switch {
	case p.needStat:
		data = NeedsStatInfo
	case p.needInum:
		data = NeedsInodeNumber
	case p.needType:
		data = NeedsType
	}
	inherent := NeedsUnknown
	switch {
	case p.is(fnExec) || p.is(fnExecdir) || p.is(fnOk) || p.is(fnOkdir): // predicate_uses_exec
		inherent = NeedsImmediateExec
		if p.args.exec.multiple {
			inherent = NeedsEventualExec
		}
	case p.is(fnFprintf):
		inherent = p.cost // computed by the -printf parser (Task 7)
	default:
		if c, ok := costLookup[p.id]; ok {
			inherent = c
		}
	}
	return max(inherent, data)
}

// estimateCosts is tree.c estimate_costs.
func estimateCosts(t *Predicate) {
	if t != nil {
		estimateCosts(t.right)
		estimateCosts(t.left)
		t.cost = getPredCost(t)
	}
}

// getRate is tree.c getrate.
func getRate(p *Predicate) float32 {
	if p == nil {
		return 1.0
	}
	return p.estSuccessRate
}

// calculateDerivedRates is tree.c calculate_derived_rates. C computes
// 1.0 - rate in double, which is exact for a float operand, so the float32
// result is the same.
func calculateDerivedRates(p *Predicate) float32 {
	if p.right != nil {
		calculateDerivedRates(p.right)
	}
	if p.left != nil {
		calculateDerivedRates(p.left)
	}
	switch p.typ {
	case UniOp:
		p.estSuccessRate = 1.0 - p.right.estSuccessRate
	case BiOp:
		var rate float32
		switch {
		case p.is(fnAnd):
			rate = getRate(p.right) * getRate(p.left)
		case p.is(fnComma):
			rate = 1.0
		case p.is(fnOr):
			rate = getRate(p.right) + getRate(p.left)
		}
		p.estSuccessRate = constrainRate(rate)
	case OpenParen, CloseParen:
		p.estSuccessRate = 1.0
	}
	return p.estSuccessRate
}

// predlist is tree.c struct predlist: BI_OP nodes chained through left.
type predlist struct{ head, tail *Predicate }

// predlistInsert is tree.c predlist_insert (prepends).
func predlistInsert(list *predlist, curr *Predicate, pprev **Predicate) {
	*pprev = curr.left
	curr.left = list.head
	list.head = curr
	if list.tail == nil {
		list.tail = list.head
	}
}

// predCostCompare is tree.c pred_cost_compare.
func predCostCompare(p1, p2 *Predicate, wantFailure bool) int {
	if p1.cost == p2.cost {
		switch {
		case p1.estSuccessRate == p2.estSuccessRate:
			return 0
		case wantFailure:
			if p1.estSuccessRate < p2.estSuccessRate {
				return -1
			}
			return 1
		default:
			if p1.estSuccessRate < p2.estSuccessRate {
				return 1
			}
			return -1
		}
	}
	if p1.cost < p2.cost {
		return -1
	}
	return 1
}

// predlistMergeSort is tree.c predlist_merge_sort (an insertion sort that
// inserts after the first node comparing >= 0, as GNU does).
func (r *Run) predlistMergeSort(list *predlist, last **Predicate) {
	if list.head == nil {
		return
	}
	if r.opt.Debug&DebugTreeOpt != 0 {
		r.debugf("%s:\n", "predlist before merge sort")
		r.printTree(list.head, 2)
	}
	calculateDerivedRates(list.head)
	var nl predlist
	for list.head != nil {
		q := list.head
		list.head = list.head.left
		q.left = nil
		var p *Predicate
		for p = nl.head; p != nil; p = p.left {
			wantFailure := p.prec != OrPrec
			if predCostCompare(p.right, q.right, wantFailure) >= 0 {
				break
			}
		}
		if p != nil {
			q.left = p.left
			if q.left == nil {
				nl.tail = q
			}
			p.left = q
		} else {
			q.left = nl.head
			nl.head = q
			if nl.tail == nil {
				nl.tail = q
			}
		}
	}
	if r.opt.Debug&DebugTreeOpt != 0 {
		r.debugf("%s:\n", "predlist after merge sort")
		r.printTree(nl.head, 2)
	}
	calculateDerivedRates(nl.head)
	mergePred(nl.head, nl.tail, last)
	*list = predlist{}
}

// mergeLists is tree.c merge_lists.
func (r *Run) mergeLists(lists *[numCosts]predlist, nameList, regexList *predlist, last **Predicate) {
	r.predlistMergeSort(nameList, last)
	r.predlistMergeSort(regexList, last)
	for i := range lists {
		r.predlistMergeSort(&lists[i], last)
	}
}

// predicateIsCostFree is tree.c predicate_is_cost_free.
func (r *Run) predicateIsCostFree(p *Predicate) bool {
	if p.is(fnName) || p.is(fnPath) || p.is(fnIname) || p.is(fnIpath) {
		return true
	}
	if r.opt.OptimisationLevel > 0 {
		if p.is(fnAnd) || p.is(fnNegate) || p.is(fnComma) || p.is(fnOr) {
			return false
		}
		return p.cost == NeedsNothing
	}
	return false
}

// optExpr is tree.c opt_expr: normalise the tree into a left-linked chain
// of one operator and reorder each group between side effects. It returns
// whether the tree has side effects.
func (r *Run) optExpr(treep **Predicate) bool {
	if treep == nil || *treep == nil {
		return false
	}
	var regexList, nameList predlist
	var cbo [numCosts]predlist
	hasSideEffects := false

	prevp := treep
	prevPrec := AndPrec
	curr := *prevp
	for curr.left != nil {
		prevp = &curr.left
		prevPrec = curr.prec
		curr = curr.left
	}
	if curr.typ != BiOp {
		setNewParent(curr, prevPrec, prevp)
	}
	if r.opt.Debug&(DebugExpressionTree|DebugTreeOpt) != 0 {
		r.debugf("Normalized Eval Tree:\n")
		r.printTree(*treep, 0)
	}

	prevp = treep
	biopPrec := NoPrec
	if *prevp != nil && (*prevp).typ == BiOp {
		biopPrec = (*prevp).prec
	}
	for {
		curr = *prevp
		if curr == nil {
			break
		}
		if curr.typ == BiOp && curr.prec != biopPrec {
			curr = setNewParent(curr, biopPrec, prevp)
		}
		rp := curr.right
		switch rp.typ {
		case NoType, PrimaryType:
			if biopPrec == CommaPrec || rp.sideEffects {
				break
			}
			if r.predicateIsCostFree(rp) {
				if r.opt.Debug&DebugTreeOpt != 0 {
					r.debugf("-O%d: promoting cheap predicate ", r.opt.OptimisationLevel)
					r.printPredicate(rp)
					r.debugf(" into name_list\n")
				}
				predlistInsert(&nameList, curr, prevp)
				continue
			}
			if rp.is(fnRegex) {
				predlistInsert(&regexList, curr, prevp)
				continue
			}
			reorder := (r.opt.OptimisationLevel > 1 &&
				(rp.cost == NeedsType || rp.cost == NeedsInodeNumber) && !rp.needStat) ||
				r.opt.OptimisationLevel > 2
			if reorder {
				if r.opt.Debug&DebugTreeOpt != 0 {
					r.debugf("-O%d: categorising predicate ", r.opt.OptimisationLevel)
					r.printPredicate(rp)
					r.debugf(" by cost (%s)\n", costName(rp.cost))
				}
				predlistInsert(&cbo[rp.cost], curr, prevp)
				continue
			}
		case UniOp:
			rp.sideEffects = r.optExpr(&rp.right)
		case BiOp:
			// GNU's "curr->pred_right->side_effects = opt_expr
			// (&curr->pred_right)" marks the subtree root from before
			// the call (the oracle's build evaluates the left side
			// first), even when the call puts another node there.
			rp.sideEffects = r.optExpr(&curr.right)
		default:
			r.fatalf(nil, "oops -- invalid expression type!  This is probably a bug.")
		}
		if curr.right.sideEffects {
			r.mergeLists(&cbo, &nameList, &regexList, prevp)
			hasSideEffects = true
		}
		prevp = &curr.left
	}
	r.mergeLists(&cbo, &nameList, &regexList, prevp)
	return hasSideEffects
}

// subtreeHasSideEffects is tree.c subtree_has_side_effects.
func subtreeHasSideEffects(p *Predicate) bool {
	return p != nil && (p.sideEffects || subtreeHasSideEffects(p.left) || subtreeHasSideEffects(p.right))
}

// worstCost is tree.c worst_cost.
func worstCost(p *Predicate) Cost {
	if p == nil {
		return 0
	}
	return max(worstCost(p.left), worstCost(p.right), p.cost)
}

// considerArmSwap is tree.c consider_arm_swap.
func (r *Run) considerArmSwap(p *Predicate) bool {
	reason := ""
	var pl, pr **Predicate
	switch {
	case p.typ != BiOp:
		reason = "Not a binary operation"
	case p.left == nil || p.right == nil:
		reason = "Doesn't have two arms"
	case p.left.right == nil:
		reason = "Left arm has no child on RHS"
	}
	if reason == "" {
		pr, pl = &p.right, &p.left.right
		if subtreeHasSideEffects(*pl) {
			reason = "Left subtree has side-effects"
		} else if subtreeHasSideEffects(*pr) {
			reason = "Right subtree has side-effects"
		}
	}
	var lc, rc Cost
	if reason == "" {
		lc, rc = worstCost(*pl), worstCost(*pr)
		if lc < rc {
			reason = "efficient as-is"
		}
	}
	if reason == "" {
		want := true
		if lc == rc {
			sl, sr := (*pl).estSuccessRate, (*pr).estSuccessRate
			if r.opt.Debug&DebugTreeOpt != 0 {
				r.debugf("Success rates: l=%f, r=%f\n", float64(sl), float64(sr))
			}
			switch {
			case p.is(fnOr):
				want = sr < sl
				if !want {
					reason = "Operation is OR; right success rate >= left"
				}
			case p.is(fnAnd):
				want = sr > sl
				if !want {
					reason = "Operation is AND; right success rate <= left"
				}
			default:
				want, reason = false, "Not 'AND' or 'OR'"
			}
		}
		if want {
			if r.opt.Debug&DebugTreeOpt != 0 {
				r.debugf("Performing arm swap on:\n")
				r.printTree(p, 0)
			}
			p.left.right, p.right = p.right, p.left.right // perform_arm_swap
			return true
		}
	}
	if r.opt.Debug&DebugTreeOpt != 0 {
		r.debugf("Not an arm swap candidate (%s):\n", reason)
		r.printTree(p, 0)
	}
	return false
}

// doArmSwaps is tree.c do_arm_swaps. C returns swapped after the loop,
// which is always false; so does this.
func (r *Run) doArmSwaps(p *Predicate) bool {
	if p == nil {
		return false
	}
	for {
		swapped := r.considerArmSwap(p) || r.doArmSwaps(p.left) || r.doArmSwaps(p.right)
		if !swapped {
			return false
		}
	}
}

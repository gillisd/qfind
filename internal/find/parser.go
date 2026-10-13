package find

import "golang.org/x/sys/unix"

// parser holds parser.c/tree.c file-level state for one command line.
type parser struct {
	r                 *Run
	argv              []string
	i                 int        // *arg_ptr
	preds             *Predicate // predicates
	last              *Predicate // last_pred
	firstNonOption    string     // first_nonoption_arg
	hasFirstNonOption bool
}

// shouldIssueWarnings is parser.c should_issue_warnings.
func (ps *parser) shouldIssueWarnings() bool {
	return !ps.r.opt.PosixlyCorrect && ps.r.opt.Warnings
}

// collectArg is parser.c collect_arg.
func (ps *parser) collectArg() (string, bool) {
	if ps.i >= len(ps.argv) {
		return "", false
	}
	ps.i++
	return ps.argv[ps.i-1], true
}

// getNewPred is tree.c get_new_pred: append a defaulted node.
func (ps *parser) getNewPred(e *parserEntry) *Predicate {
	p := &Predicate{entry: e, needStat: true, needType: true, cost: NeedsUnknown,
		estSuccessRate: 1.0, argText: "ThisShouldBeSetToSomethingElse", hasArgText: true}
	if ps.preds == nil {
		ps.preds = p
	} else {
		ps.last.next = p
	}
	ps.last = p
	return p
}

// getNewPredNoArg is tree.c get_new_pred_noarg.
func (ps *parser) getNewPredNoArg(e *parserEntry) *Predicate {
	p := ps.getNewPred(e)
	p.argText, p.hasArgText = "", false
	return p
}

// getNewPredChkOp is tree.c get_new_pred_chk_op: interpose an implicit -a
// after a primary or ')'.
func (ps *parser) getNewPredChkOp(e *parserEntry, arg string, hasArg bool) *Predicate {
	if ps.last != nil {
		switch ps.last.typ {
		case NoType:
			ps.r.fatalf(nil, "oops -- invalid default insertion of and!  This is probably a bug.")
		case PrimaryType, CloseParen:
			and := ps.getNewPredNoArg(lookupEntry("and"))
			and.fn, and.id, and.name = predAnd, fnAnd, "-a"
			and.typ, and.prec = BiOp, AndPrec
			and.needStat, and.needType, and.needInum = false, false, false
		}
	}
	p := ps.getNewPred(e)
	p.argText, p.hasArgText = arg, hasArg
	return p
}

// insertPrimaryWithPred is util.c insert_primary_withpred.
func (ps *parser) insertPrimaryWithPred(e *parserEntry, fn PredFunc, id FuncID, arg string, hasArg bool) *Predicate {
	p := ps.getNewPredChkOp(e, arg, hasArg)
	p.fn, p.id, p.name = fn, id, e.name
	p.typ, p.prec = PrimaryType, NoPrec
	return p
}

// insertPrimary is util.c insert_primary.
func (ps *parser) insertPrimary(e *parserEntry, arg string, hasArg bool) *Predicate {
	return ps.insertPrimaryWithPred(e, e.fn, e.id, arg, hasArg)
}

// insertPrimaryNoArg is util.c insert_primary_noarg.
func (ps *parser) insertPrimaryNoArg(e *parserEntry) *Predicate {
	return ps.insertPrimary(e, "", false)
}

// looksLikeExpression is util.c looks_like_expression.
func looksLikeExpression(arg string, leading bool) bool {
	if arg == "" {
		return false
	}
	switch arg[0] {
	case '-':
		return len(arg) > 1
	case ')', ',':
		return len(arg) == 1 && !leading
	case '!', '(':
		return len(arg) == 1
	}
	return false
}

// defaultPrints is util.c default_prints.
func defaultPrints(p *Predicate) bool {
	for ; p != nil; p = p.next {
		if p.noDefaultPrint {
			return false
		}
	}
	return true
}

// checkOptionCombinations is parser.c check_option_combinations.
func (r *Run) checkOptionCombinations(p *Predicate) {
	var del, prune bool
	for ; p != nil; p = p.next {
		del = del || p.is(fnDelete)
		prune = prune || p.is(fnPrune)
	}
	if del && prune && !r.opt.ExplicitDepth {
		r.fatalf(nil, "The -delete action automatically turns on -depth, but -prune does nothing when -depth is in effect.  If you want to carry on anyway, just explicitly use the -depth option.")
	}
}

// buildExpressionTree is tree.c build_expression_tree. args[end:] are the
// start points followed by the expression.
func (r *Run) buildExpressionTree(args []string, end int) *Predicate {
	ps := &parser{r: r, argv: args}
	i := end
	for i < len(args) && !looksLikeExpression(args[i], true) {
		r.startPoints = append(r.startPoints, args[i])
		i++
	}
	ps.i = i
	parseOpenparen(ps, lookupEntry("("))
	ps.last.name, ps.preds.artificial = "(", true

	for ps.i < len(args) {
		arg := args[ps.i]
		if !looksLikeExpression(arg, false) {
			r.errorf(nil, "paths must precede expression: `%s'", arg)
			if unix.Access(arg, unix.F_OK) == nil {
				r.errorf(nil, "possible unquoted pattern after predicate `%s'?", ps.last.name)
			}
			r.exit(1)
		}
		e := ps.findParser(arg)
		if e == nil {
			r.fatalf(nil, "unknown predicate `%s'", arg)
		}
		if e.class != argSpecialParse {
			ps.i++
		}
		old := ps.i
		if !e.parse(ps, e) {
			if ps.i < len(args) {
				if e.class == argSpecialParse && ps.i == old {
					r.fatalf(nil, "invalid predicate `%s'", arg)
				}
				r.fatalf(nil, "invalid argument `%s' to `%s'", args[ps.i], arg)
			}
			r.fatalf(nil, "missing argument to `%s'", arg)
		}
		// A parser that inserts no predicate (-daystart, -files0-from)
		// renames the previous one, as GNU does.
		ps.last.name = arg
		if ps.i != old {
			ps.last.argText, ps.last.hasArgText = args[old], true
		} else {
			ps.last.argText, ps.last.hasArgText = "", false
		}
	}

	switch {
	case ps.preds.next == nil:
		ps.preds, ps.last = nil, nil
		parsePrint(ps, lookupEntry("print"))
		ps.last.name = "-print"
	case !defaultPrints(ps.preds.next):
		ps.preds = ps.preds.next
	default:
		parseCloseparen(ps, lookupEntry(")"))
		ps.last.name, ps.last.artificial = ")", true
		parsePrint(ps, lookupEntry("print"))
		ps.last.name, ps.last.artificial = "-print", true
	}

	if r.opt.Debug&(DebugExpressionTree|DebugTreeOpt) != 0 {
		r.debugf("Predicate List:\n")
		r.printList(ps.preds)
	}
	r.preds = ps.preds
	r.checkOptionCombinations(ps.preds)

	cur := ps.preds
	tree := r.getExpr(&cur, NoPrec, nil)
	calculateDerivedRates(tree)
	if cur != nil {
		if cur.is(fnCloseParen) {
			r.fatalf(nil, "you have too many ')'")
		}
		if cur.name != "" {
			r.fatalf(nil, "unexpected extra predicate '%s'", cur.name)
		}
		r.fatalf(nil, "unexpected extra predicate")
	}
	if r.opt.Debug&(DebugExpressionTree|DebugTreeOpt) != 0 {
		r.debugf("Eval Tree:\n")
		r.printTree(tree, 0)
	}
	estimateCosts(tree)
	r.optExpr(&tree)
	if r.opt.OptimisationLevel > 1 {
		r.doArmSwaps(tree)
	}
	if r.opt.Debug&(DebugExpressionTree|DebugTreeOpt) != 0 {
		r.debugf("Optimized Eval Tree:\n")
		r.printTree(tree, 0)
		r.debugf("Optimized command line:\n")
		r.printOptlist(tree)
		r.debugf("\n")
	}
	return tree
}

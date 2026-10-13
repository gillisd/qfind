package find

// parsePrint is parser.c parse_print.
func parsePrint(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.sideEffects, p.noDefaultPrint = true, true
	p.needStat, p.needType = false, false
	p.args.out = ps.r.out.stdoutStream
	return true
}

// parsePrint0 is parser.c parse_print0 → insert_fprint(entry, NULL).
func parsePrint0(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimary(e, "", false)
	p.args.out = ps.r.out.stdoutStream
	p.sideEffects, p.noDefaultPrint = true, true
	p.needStat, p.needType = false, false
	p.estSuccessRate = 1.0
	return true
}

// parsePrune is parser.c parse_prune.
func parsePrune(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	if !ps.r.opt.DoDirFirst {
		p.needStat, p.needType = false, false
	}
	p.sideEffects, p.noDefaultPrint = true, false
	return true
}

// parseQuit is parser.c parse_quit.
func parseQuit(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.needStat, p.needType = false, false
	p.sideEffects, p.noDefaultPrint = true, false
	p.estSuccessRate = 1.0
	return true
}

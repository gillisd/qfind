package find

// parsePrintf is parser.c parse_printf.
func parsePrintf(ps *parser, e *parserEntry) bool {
	format, ok := ps.collectArg()
	if !ok {
		return false
	}
	return ps.insertFprintf(ps.r.out.stdoutStream, e, format)
}

// parseFprintf is parser.c parse_fprintf: the file opens only after the
// format was collected; without one, the filename is the bad argument.
func parseFprintf(ps *parser, e *parserEntry) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	format, ok := ps.collectArg()
	if !ok {
		ps.i-- // don't consume the filename: "invalid argument `out' to `-fprintf'"
		return false
	}
	return ps.insertFprintf(ps.openOutputFile(name), e, format)
}

// parseFprint is parser.c parse_fprint.
func parseFprint(ps *parser, e *parserEntry) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	p := ps.insertPrimary(e, name, true)
	p.args.out = ps.openOutputFile(name)
	p.sideEffects, p.noDefaultPrint = true, true
	p.needStat, p.needType = false, false
	p.estSuccessRate = 1.0
	return true
}

// parseFprint0 is parser.c parse_fprint0 → insert_fprint(entry, filename).
func parseFprint0(ps *parser, e *parserEntry) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	p := ps.insertPrimary(e, name, true)
	p.args.out = ps.openOutputFile(name)
	p.sideEffects, p.noDefaultPrint = true, true
	p.needStat, p.needType = false, false
	p.estSuccessRate = 1.0
	return true
}

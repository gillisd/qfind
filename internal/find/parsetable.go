package find

// argClass is enum arg_type (defs.h).
type argClass uint8

const (
	argOption argClass = iota + 1
	argPositionalOption
	argPunctuation
	argTest
	argAction
	argSpecialParse
	argNoop
)

// parseFunc is PARSE_FUNC: it consumes its arguments from ps.argv at ps.i.
type parseFunc func(ps *parser, e *parserEntry) bool

// parserEntry is struct parser_table (defs.h).
type parserEntry struct {
	class argClass
	name  string // parser_name, without the first '-'
	parse parseFunc
	fn    PredFunc
	id    FuncID
}

var (
	parseTable        []parserEntry
	parseEntryNewerXY parserEntry
)

// parseLater stands in for a primary whose port lands in a later task. A
// special parse (-newerXY) has not consumed its own argument yet.
func parseLater(ps *parser, e *parserEntry) bool {
	arg := ps.argv[ps.i-1]
	if e.class == argSpecialParse {
		arg = ps.argv[ps.i]
	}
	ps.r.fatalf(nil, "unknown predicate `%s'", arg)
	return false
}

// init fills parser.c parse_table[]; a var initialiser would form an
// initialisation cycle through parseNoop, which scans the table.
func init() {
	parseTable = []parserEntry{
		{argOption, "d", parseD, nil, fnNone},
		{argOption, "depth", parseDepth, nil, fnNone},
		{argOption, "files0-from", parseFiles0From, nil, fnNone},
		{argOption, "ignore_readdir_race", parseIgnoreRace, nil, fnNone},
		{argOption, "maxdepth", parseMaxdepth, nil, fnNone},
		{argOption, "mindepth", parseMindepth, nil, fnNone},
		{argOption, "mount", parseMount, nil, fnNone},
		{argOption, "noleaf", parseNoleaf, nil, fnNone},
		{argOption, "noignore_readdir_race", parseNoignoreRace, nil, fnNone},
		{argOption, "xdev", parseXdev, nil, fnNone},
		{argOption, "help", parseHelp, nil, fnNone},
		{argOption, "-help", parseHelp, nil, fnNone},
		{argOption, "version", parseVersion, nil, fnNone},
		{argOption, "-version", parseVersion, nil, fnNone},
		{argPositionalOption, "daystart", parseDaystart, nil, fnNone},
		{argPositionalOption, "follow", parseFollow, nil, fnNone},
		{argPositionalOption, "nowarn", parseNowarn, nil, fnNone},
		{argPositionalOption, "regextype", parseLater, nil, fnNone}, // Task 10
		{argPositionalOption, "warn", parseWarn, nil, fnNone},
		{argPunctuation, "!", parseNegate, predNegate, fnNegate},
		{argPunctuation, "not", parseNegate, predNegate, fnNegate},
		{argPunctuation, "(", parseOpenparen, predOpenparen, fnOpenParen},
		{argPunctuation, ")", parseCloseparen, predCloseparen, fnCloseParen},
		{argPunctuation, ",", parseComma, predComma, fnComma},
		{argPunctuation, "a", parseAnd, predAnd, fnAnd},
		{argPunctuation, "and", parseAnd, predAnd, fnAnd},
		{argPunctuation, "o", parseOr, predOr, fnOr},
		{argPunctuation, "or", parseOr, predOr, fnOr},
		{argTest, "amin", parseLater, nil, fnAmin},       // Task 12
		{argTest, "anewer", parseLater, nil, fnAnewer},   // Task 12
		{argTest, "atime", parseLater, nil, fnAtime},     // Task 12
		{argTest, "cmin", parseLater, nil, fnCmin},       // Task 12
		{argTest, "cnewer", parseLater, nil, fnCnewer},   // Task 12
		{argTest, "ctime", parseLater, nil, fnCtime},     // Task 12
		{argTest, "context", parseLater, nil, fnContext}, // Task 13
		{argTest, "empty", parseEmpty, predEmpty, fnEmpty},
		{argTest, "executable", parseLater, nil, fnExecutable}, // Task 13
		{argTest, "false", parseFalse, predFalse, fnFalse},
		{argTest, "fstype", parseLater, nil, fnFstype}, // Task 13
		{argTest, "gid", parseGid, predGid, fnGid},
		{argTest, "group", parseGroup, predGroup, fnGroup},
		{argTest, "ilname", parseIlname, predIlname, fnIlname},
		{argTest, "iname", parseIname, predIname, fnIname},
		{argTest, "inum", parseInum, predInum, fnInum},
		{argTest, "ipath", parseIpath, predIpath, fnIpath},
		{argTest, "iregex", parseLater, nil, fnRegex}, // Task 10
		{argTest, "iwholename", parseIwholename, predIpath, fnIpath},
		{argTest, "links", parseLinks, predLinks, fnLinks},
		{argTest, "lname", parseLname, predLname, fnLname},
		{argTest, "mmin", parseLater, nil, fnMmin},   // Task 12
		{argTest, "mtime", parseLater, nil, fnMtime}, // Task 12
		{argTest, "name", parseName, predName, fnName},
		{argTest, "newer", parseLater, nil, fnNewer}, // Task 12
		{argTest, "nogroup", parseNogroup, predNogroup, fnNogroup},
		{argTest, "nouser", parseNouser, predNouser, fnNouser},
		{argTest, "path", parsePath, predPath, fnPath},
		{argTest, "perm", parseLater, nil, fnPerm},         // Task 13
		{argTest, "readable", parseLater, nil, fnReadable}, // Task 13
		{argTest, "regex", parseLater, nil, fnRegex},       // Task 10
		{argTest, "samefile", parseSamefile, predSamefile, fnSamefile},
		{argTest, "size", parseSize, predSize, fnSize},
		{argTest, "true", parseTrue, predTrue, fnTrue},
		{argTest, "type", parseType, predType, fnType},
		{argTest, "uid", parseUid, predUid, fnUid},
		{argTest, "used", parseLater, nil, fnUsed}, // Task 12
		{argTest, "user", parseUser, predUser, fnUser},
		{argTest, "wholename", parseWholename, predPath, fnPath},
		{argTest, "writable", parseLater, nil, fnWritable}, // Task 13
		{argTest, "xtype", parseXtype, predXtype, fnXtype},
		{argAction, "delete", parseLater, nil, fnDelete},   // Task 9
		{argAction, "exec", parseLater, nil, fnExec},       // Task 8
		{argAction, "execdir", parseLater, nil, fnExecdir}, // Task 8
		{argAction, "fls", parseLater, nil, fnFls},         // Task 9
		{argAction, "fprint", parseFprint, predPrint, fnFprint},
		{argAction, "fprint0", parseFprint0, predPrint0, fnFprint0},
		{argAction, "fprintf", parseFprintf, predFprintf, fnFprintf},
		{argAction, "ls", parseLater, nil, fnLs},       // Task 9
		{argAction, "ok", parseLater, nil, fnOk},       // Task 8
		{argAction, "okdir", parseLater, nil, fnOkdir}, // Task 8
		{argAction, "print", parsePrint, predPrint, fnPrint},
		{argAction, "print0", parsePrint0, predPrint0, fnPrint0},
		{argAction, "printf", parsePrintf, predFprintf, fnFprintf},
		{argAction, "prune", parsePrune, predPrune, fnPrune},
		{argAction, "quit", parseQuit, predQuit, fnQuit},
		{argNoop, "--noop", nil, predTrue, fnTrue},
	}
	parseEntryNewerXY = parserEntry{argSpecialParse, "newerXY", parseLater, nil, fnNewerXY} // Task 12
}

// lookupEntry finds a table entry by its parser_name.
func lookupEntry(name string) *parserEntry {
	for i := range parseTable {
		if parseTable[i].name == name {
			return &parseTable[i]
		}
	}
	return nil
}

// findParser is parser.c find_parser.
func (ps *parser) findParser(arg string) *parserEntry {
	if len(arg) == 8 && arg[:6] == "-newer" {
		return ps.foundParser(arg, &parseEntryNewerXY)
	}
	name := arg
	if name != "" && name[0] == '-' {
		name = name[1:]
	}
	e := lookupEntry(name)
	if e == nil {
		return nil
	}
	if name != arg && e.class == argPunctuation &&
		(name == "!" || name == "," || name == "(" || name == ")") {
		ps.r.errorf(nil, "warning: operator '%s' (with leading dash '-') will no longer be accepted in future findutils releases!", arg)
	}
	return ps.foundParser(arg, e)
}

// foundParser is parser.c found_parser: the placement warning for global
// options after a non-option argument.
func (ps *parser) foundParser(arg string, e *parserEntry) *parserEntry {
	if e.class == argPositionalOption {
		return e
	}
	if e.class == argNoop {
		return nil
	}
	if e.class == argOption {
		if ps.hasFirstNonOption && ps.shouldIssueWarnings() {
			ps.r.errorf(nil, "warning: you have specified the global option %s after the argument %s, but global options are not positional, i.e., %s affects tests specified before it as well as those specified after it.  Please specify global options before other arguments.", arg, ps.firstNonOption, arg)
		}
		return e
	}
	if !ps.hasFirstNonOption {
		ps.firstNonOption, ps.hasFirstNonOption = arg, true
	}
	return e
}

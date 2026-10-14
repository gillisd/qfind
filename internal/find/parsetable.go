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
		{argPositionalOption, "regextype", parseRegextype, nil, fnNone},
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
		{argTest, "amin", parseXmin, predAmin, fnAmin},
		{argTest, "anewer", parseAnewer, predAnewer, fnAnewer},
		{argTest, "atime", parseTime, predAtime, fnAtime},
		{argTest, "cmin", parseXmin, predCmin, fnCmin},
		{argTest, "cnewer", parseCnewer, predCnewer, fnCnewer},
		{argTest, "ctime", parseTime, predCtime, fnCtime},
		{argTest, "context", parseContext, predContext, fnContext},
		{argTest, "empty", parseEmpty, predEmpty, fnEmpty},
		{argTest, "executable", parseAccesscheck, predExecutable, fnExecutable},
		{argTest, "false", parseFalse, predFalse, fnFalse},
		{argTest, "fstype", parseFstype, predFstype, fnFstype},
		{argTest, "gid", parseGid, predGid, fnGid},
		{argTest, "group", parseGroup, predGroup, fnGroup},
		{argTest, "ilname", parseIlname, predIlname, fnIlname},
		{argTest, "iname", parseIname, predIname, fnIname},
		{argTest, "inum", parseInum, predInum, fnInum},
		{argTest, "ipath", parseIpath, predIpath, fnIpath},
		{argTest, "iregex", parseIregex, predRegex, fnRegex},
		{argTest, "iwholename", parseIwholename, predIpath, fnIpath},
		{argTest, "links", parseLinks, predLinks, fnLinks},
		{argTest, "lname", parseLname, predLname, fnLname},
		{argTest, "mmin", parseXmin, predMmin, fnMmin},
		{argTest, "mtime", parseTime, predMtime, fnMtime},
		{argTest, "name", parseName, predName, fnName},
		{argTest, "newer", parseNewer, predNewer, fnNewer},
		{argTest, "nogroup", parseNogroup, predNogroup, fnNogroup},
		{argTest, "nouser", parseNouser, predNouser, fnNouser},
		{argTest, "path", parsePath, predPath, fnPath},
		{argTest, "perm", parsePerm, predPerm, fnPerm},
		{argTest, "readable", parseAccesscheck, predReadable, fnReadable},
		{argTest, "regex", parseRegex, predRegex, fnRegex},
		{argTest, "samefile", parseSamefile, predSamefile, fnSamefile},
		{argTest, "size", parseSize, predSize, fnSize},
		{argTest, "true", parseTrue, predTrue, fnTrue},
		{argTest, "type", parseType, predType, fnType},
		{argTest, "uid", parseUid, predUid, fnUid},
		{argTest, "used", parseUsed, predUsed, fnUsed},
		{argTest, "user", parseUser, predUser, fnUser},
		{argTest, "wholename", parseWholename, predPath, fnPath},
		{argTest, "writable", parseAccesscheck, predWritable, fnWritable},
		{argTest, "xtype", parseXtype, predXtype, fnXtype},
		{argAction, "delete", parseDelete, predDelete, fnDelete},
		{argAction, "exec", parseExec, predExec, fnExec},
		{argAction, "execdir", parseExecdir, predExec, fnExecdir},
		{argAction, "fls", parseFls, predLs, fnFls},
		{argAction, "fprint", parseFprint, predPrint, fnFprint},
		{argAction, "fprint0", parseFprint0, predPrint0, fnFprint0},
		{argAction, "fprintf", parseFprintf, predFprintf, fnFprintf},
		{argAction, "ls", parseLs, predLs, fnLs},
		{argAction, "ok", parseOk, predExec, fnOk},
		{argAction, "okdir", parseOkdir, predExec, fnOkdir},
		{argAction, "print", parsePrint, predPrint, fnPrint},
		{argAction, "print0", parsePrint0, predPrint0, fnPrint0},
		{argAction, "printf", parsePrintf, predFprintf, fnFprintf},
		{argAction, "prune", parsePrune, predPrune, fnPrune},
		{argAction, "quit", parseQuit, predQuit, fnQuit},
		{argNoop, "--noop", nil, predTrue, fnTrue},
	}
	parseEntryNewerXY = parserEntry{argSpecialParse, "newerXY", parseNewerXY, predNewerXY, fnNewerXY}
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

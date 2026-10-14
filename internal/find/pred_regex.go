package find

import (
	"strings"

	"qfind/internal/regex"
)

// regexTypes is lib/regextype.c regex_map[], in its order (the order of the
// "valid types are" list). ed, sed and posix-egrep are gnulib regex.h
// aliases of posix-basic and egrep.
var regexTypes = []struct {
	name string
	syn  regex.Syntax
}{
	{"findutils-default", regex.SyntaxEmacs | regex.DotNewline},
	{"awk", regex.SyntaxAwk},
	{"ed", regex.SyntaxPosixBasic},
	{"egrep", regex.SyntaxEgrep},
	{"emacs", regex.SyntaxEmacs},
	{"gnu-awk", regex.SyntaxGNUAwk},
	{"grep", regex.SyntaxGrep},
	{"posix-awk", regex.SyntaxPosixAwk},
	{"posix-basic", regex.SyntaxPosixBasic},
	{"posix-egrep", regex.SyntaxEgrep},
	{"posix-extended", regex.SyntaxPosixExtended},
	{"posix-minimal-basic", regex.SyntaxPosixMinimalBasic},
	{"sed", regex.SyntaxPosixBasic},
}

// parseRegextype is parser.c parse_regextype + lib/regextype.c get_regex_type.
func parseRegextype(ps *parser, e *parserEntry) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	for _, t := range regexTypes {
		if t.name == name {
			ps.r.opt.RegexSyntax = t.syn
			return parseNoop(ps, e)
		}
	}
	quoted := make([]string, len(regexTypes))
	for i, t := range regexTypes {
		quoted[i] = ps.r.quote(t.name)
	}
	ps.r.fatalf(nil, "Unknown regular expression type %s; valid types are %s.", ps.r.quote(name), strings.Join(quoted, ", "))
	return false
}

// insertRegex is parser.c insert_regex: re_compile_pattern under the given
// syntax; a compile failure is fatal with glibc's message.
func (ps *parser) insertRegex(e *parserEntry, syn regex.Syntax) bool {
	rx, ok := ps.collectArg()
	if !ok {
		return false
	}
	p := ps.insertPrimaryWithPred(e, predRegex, fnRegex, rx, true)
	p.needStat, p.needType = false, false
	re, err := regex.Compile(rx, syn, ps.r.loc.UTF8)
	if err != nil {
		ps.r.fatalf(nil, "failed to compile regular expression '%s': %s", rx, err.Error())
		return false
	}
	p.args.regex = re
	p.estSuccessRate = estimatePatternMatchRate(rx, true)
	return true
}

func parseRegex(ps *parser, e *parserEntry) bool { return ps.insertRegex(e, ps.r.opt.RegexSyntax) }

// parseIregex is parser.c parse_iregex: RE_ICASE on top of -regextype.
func parseIregex(ps *parser, e *parserEntry) bool {
	return ps.insertRegex(e, ps.r.opt.RegexSyntax|regex.ICase)
}

// predRegex is pred.c pred_regex: re_match(…) == len, a whole-path match.
func predRegex(r *Run, e *Entry, p *Predicate) bool { return p.args.regex.FullMatch(e.path) }

package find

import "strings"

// checkNameArg is parser.c check_name_arg.
func (ps *parser) checkNameArg(pred, alt, arg string) {
	if ps.shouldIssueWarnings() && strings.Contains(arg, "/") && arg != "/" {
		r := ps.r
		r.errorf(nil, "warning: %s matches against basenames only, but the given pattern contains a directory separator (%s), thus the expression will evaluate to false all the time.  Did you mean %s?",
			r.quote(pred), r.quoteN("/"), r.quoteN(alt))
	}
}

// insertName is parser.c parse_name and parse_iname.
func (ps *parser) insertName(e *parserEntry, pred, alt string) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	ps.checkNameArg(pred, alt, name)
	p := ps.insertPrimary(e, name, true)
	p.needStat, p.needType = false, false
	p.args.str = name
	p.estSuccessRate = estimatePatternMatchRate(name, false)
	return true
}

func parseName(ps *parser, e *parserEntry) bool  { return ps.insertName(e, "-name", "-wholename") }
func parseIname(ps *parser, e *parserEntry) bool { return ps.insertName(e, "-iname", "-iwholename") }

// insertPathCheck is parser.c insert_path_check. The table entry already
// carries pred_path or pred_ipath.
func (ps *parser) insertPathCheck(e *parserEntry, predName string) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	p := ps.insertPrimary(e, name, true)
	p.needStat, p.needType = false, false
	p.args.str = name
	p.estSuccessRate = estimatePatternMatchRate(name, false)
	if !ps.r.opt.PosixlyCorrect && !ps.r.isFeasiblePathArgument(name, e.id == fnIpath) {
		ps.r.errorf(nil, "warning: -%s %s will not match anything because it ends with /.", predName, name)
		p.estSuccessRate = 1.0e-8
	}
	return true
}

func parsePath(ps *parser, e *parserEntry) bool       { return ps.insertPathCheck(e, "path") }
func parseWholename(ps *parser, e *parserEntry) bool  { return ps.insertPathCheck(e, "wholename") }
func parseIpath(ps *parser, e *parserEntry) bool      { return ps.insertPathCheck(e, "ipath") }
func parseIwholename(ps *parser, e *parserEntry) bool { return ps.insertPathCheck(e, "iwholename") }

// insertLname is parser.c parse_lname and parse_ilname: the default
// need_stat and need_type stay, since match_lname reads st_mode.
func (ps *parser) insertLname(e *parserEntry) bool {
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	p := ps.insertPrimary(e, name, true)
	p.args.str = name
	p.estSuccessRate = 0.1 * estimatePatternMatchRate(name, false)
	return true
}

func parseLname(ps *parser, e *parserEntry) bool  { return ps.insertLname(e) }
func parseIlname(ps *parser, e *parserEntry) bool { return ps.insertLname(e) }

// isFeasiblePathArgument is parser.c is_feasible_path_argument: a pattern
// ending in '/' can only match a start point typed that way.
func (r *Run) isFeasiblePathArgument(arg string, fold bool) bool {
	if strings.HasSuffix(arg, "/") {
		return r.matchesStartPoint(arg, fold)
	}
	return true
}

// matchesStartPoint is tree.c matches_start_point. With -files0-from,
// start_points holds no names, so "." stands in.
func (r *Run) matchesStartPoint(glob string, fold bool) bool {
	if len(r.startPoints) == 0 {
		return fnmatchRun(r, glob, ".", fold)
	}
	for _, s := range r.startPoints {
		if fnmatchRun(r, glob, s, fold) {
			return true
		}
	}
	return false
}

// estimatePatternMatchRate is parser.c estimate_pattern_match_rate.
func estimatePatternMatchRate(pattern string, isRegex bool) float32 {
	if strings.ContainsAny(pattern, "*?[") || (isRegex && strings.Contains(pattern, ".")) {
		return 0.8
	}
	return 0.1
}

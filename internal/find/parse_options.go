package find

import (
	"math"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// parseTrue is parser.c parse_true.
func parseTrue(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.needStat, p.needType = false, false
	p.estSuccessRate = 1.0
	return true
}

// insertFalse is parser.c insert_false (also used by -fstype substitution).
func (ps *parser) insertFalse() bool {
	e := ps.findParser("false")
	p := ps.insertPrimaryNoArg(e)
	p.needStat, p.needType = false, false
	p.sideEffects, p.noDefaultPrint = false, false
	p.estSuccessRate = 0
	return true
}

// parseFalse is parser.c parse_false.
func parseFalse(ps *parser, e *parserEntry) bool { return ps.insertFalse() }

// parseNoop is parser.c parse_noop: a -true node later renamed to the option.
func parseNoop(ps *parser, e *parserEntry) bool {
	for i := range parseTable {
		if parseTable[i].class == argNoop {
			return parseTrue(ps, &parseTable[i])
		}
	}
	return false
}

// parseD is parser.c parse_d.
func parseD(ps *parser, e *parserEntry) bool {
	if ps.shouldIssueWarnings() {
		ps.r.errorf(nil, "warning: the -d option is deprecated; please use -depth instead, because the latter is a POSIX-compliant feature.")
	}
	return parseDepth(ps, e)
}

// parseDepth is parser.c parse_depth.
func parseDepth(ps *parser, e *parserEntry) bool {
	ps.r.opt.DoDirFirst, ps.r.opt.ExplicitDepth = false, true
	return parseNoop(ps, e)
}

// parseDaystart is parser.c parse_daystart: local midnight today, once.
func parseDaystart(ps *parser, e *parserEntry) bool {
	o := &ps.r.opt
	if !o.FullDays {
		o.CurDayStart.Sec += 86400
		o.CurDayStart.Nsec = 0
		t := time.Unix(o.CurDayStart.Sec, 0).In(ps.r.tz)
		o.CurDayStart.Sec -= int64(t.Second() + t.Minute()*60 + t.Hour()*3600)
		o.FullDays = true
	}
	return true // inserts no predicate: the caller renames the previous one
}

// parseFollow is parser.c parse_follow.
func parseFollow(ps *parser, e *parserEntry) bool {
	ps.r.setFollowState(SymlinkAlwaysDeref)
	return parseNoop(ps, e)
}

// parseWarn, parseNowarn, parseIgnoreRace, parseNoignoreRace, parseMount,
// parseXdev and parseNoleaf are their parser.c namesakes: set the option,
// then insert a -true node.
func parseWarn(ps *parser, e *parserEntry) bool {
	ps.r.opt.Warnings = true
	return parseNoop(ps, e)
}

func parseNowarn(ps *parser, e *parserEntry) bool {
	ps.r.opt.Warnings = false
	return parseNoop(ps, e)
}

func parseIgnoreRace(ps *parser, e *parserEntry) bool {
	ps.r.opt.IgnoreReaddirRace = true
	return parseNoop(ps, e)
}

func parseNoignoreRace(ps *parser, e *parserEntry) bool {
	ps.r.opt.IgnoreReaddirRace = false
	return parseNoop(ps, e)
}

func parseMount(ps *parser, e *parserEntry) bool {
	ps.r.opt.Mount = true
	return parseNoop(ps, e)
}

func parseXdev(ps *parser, e *parserEntry) bool {
	ps.r.opt.Xdev = true
	return parseNoop(ps, e)
}

func parseNoleaf(ps *parser, e *parserEntry) bool {
	ps.r.opt.NoLeafCheck = true
	return parseNoop(ps, e)
}

// parseHelp and parseVersion are parser.c parse_help and parse_version;
// both exit.
func parseHelp(ps *parser, e *parserEntry) bool {
	ps.r.usage()
	return true
}

func parseVersion(ps *parser, e *parserEntry) bool {
	ps.r.version()
	return true
}

// parseFiles0From is parser.c parse_files0_from: no predicate is inserted.
func parseFiles0From(ps *parser, e *parserEntry) bool {
	name, ok := ps.collectArg()
	if ok {
		ps.r.opt.Files0From, ps.r.opt.HasFiles0From = name, true
	}
	return ok
}

func parseMaxdepth(ps *parser, e *parserEntry) bool { return ps.insertDepthspec(e, &ps.r.opt.MaxDepth) }
func parseMindepth(ps *parser, e *parserEntry) bool { return ps.insertDepthspec(e, &ps.r.opt.MinDepth) }

// insertDepthspec is parser.c insert_depthspec with lib/safe-atoi.c: a
// string of digits only, whose strtol value must fit an int.
func (ps *parser) insertDepthspec(e *parserEntry, limit *int) bool {
	option := ps.argv[ps.i-1]
	s, ok := ps.collectArg()
	if !ok {
		return false
	}
	n := len(s) - len(strings.TrimLeft(s, "0123456789"))
	if n > 0 && n == len(s) {
		v, err := strconv.ParseInt(s, 10, 64)
		switch {
		case err != nil: // strtol overflow: LONG_MAX with ERANGE
			ps.r.fatalf(syscall.ERANGE, "%s", s)
		case v == math.MaxInt64: // LONG_MAX itself: errno is still 0
			ps.r.fatalf(nil, "%s", s)
		case v > math.MaxInt32:
			ps.r.fatalf(syscall.ERANGE, "%s", s)
		}
		*limit = int(v)
		return parseNoop(ps, e)
	}
	ps.r.fatalf(nil, "Expected a positive decimal integer argument to %s, but got %s", option, ps.r.quote(s))
	return false
}

// parseAnd is parser.c parse_and.
func parseAnd(ps *parser, e *parserEntry) bool {
	p := ps.getNewPredNoArg(e)
	p.fn, p.id, p.typ, p.prec = predAnd, fnAnd, BiOp, AndPrec
	p.needStat, p.needType = false, false
	return true
}

// parseOr is parser.c parse_or.
func parseOr(ps *parser, e *parserEntry) bool {
	p := ps.getNewPredNoArg(e)
	p.fn, p.id, p.typ, p.prec = predOr, fnOr, BiOp, OrPrec
	p.needStat, p.needType = false, false
	return true
}

// parseComma is parser.c parse_comma.
func parseComma(ps *parser, e *parserEntry) bool {
	p := ps.getNewPredNoArg(e)
	p.fn, p.id, p.typ, p.prec = predComma, fnComma, BiOp, CommaPrec
	p.needStat, p.needType = false, false
	p.estSuccessRate = 1.0
	return true
}

// parseNegate is parser.c parse_negate.
func parseNegate(ps *parser, e *parserEntry) bool {
	p := ps.getNewPredChkOp(e, "", false)
	p.fn, p.id, p.typ, p.prec = predNegate, fnNegate, UniOp, NegatePrec
	p.needStat, p.needType = false, false
	return true
}

// parseOpenparen is parser.c parse_openparen.
func parseOpenparen(ps *parser, e *parserEntry) bool {
	p := ps.getNewPredChkOp(e, "", false)
	p.fn, p.id, p.typ, p.prec = predOpenparen, fnOpenParen, OpenParen, NoPrec
	p.needStat, p.needType = false, false
	return true
}

// parseCloseparen is parser.c parse_closeparen.
func parseCloseparen(ps *parser, e *parserEntry) bool {
	p := ps.getNewPredNoArg(e)
	p.fn, p.id, p.typ, p.prec = predCloseparen, fnCloseParen, CloseParen, NoPrec
	p.needStat, p.needType = false, false
	return true
}

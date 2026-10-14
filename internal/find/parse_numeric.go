package find

import (
	"golang.org/x/sys/unix"

	"qfind/internal/libc"
)

// compKind is enum comparison_type (defs.h).
type compKind uint8

const (
	compGT compKind = iota
	compLT
	compEQ
)

// getCompType is parser.c get_comp_type: '+' greater, '-' less, else equal.
func getCompType(s string) (compKind, string) {
	if s != "" {
		switch s[0] {
		case '+':
			return compGT, s[1:]
		case '-':
			return compLT, s[1:]
		}
	}
	return compEQ, s
}

// compareNum is pred.c compare_num.
func compareNum(kind compKind, l, v uint64) bool {
	switch kind {
	case compGT:
		return v > l
	case compLT:
		return v < l
	}
	return v == l
}

type numInfo struct {
	kind compKind
	val  uint64
}

// insertNum is parser.c insert_num (-inum, -links, -uid, -gid).
func (ps *parser) insertNum(e *parserEntry) *Predicate {
	numstr, ok := ps.collectArg()
	if !ok {
		return nil
	}
	kind, rest := getCompType(numstr)
	v, ok := libc.Xstrtoumax(rest)
	if !ok {
		ps.r.fatalf(nil, "non-numeric argument to %s: %s", ps.argv[ps.i-2], ps.r.quote(numstr))
	}
	p := ps.insertPrimary(e, numstr, true)
	p.args.num = numInfo{kind, v}
	if ps.r.opt.Debug&DebugExpressionTree != 0 {
		names := [...][2]string{compGT: {"gt", " >"}, compLT: {"lt", " <"}, compEQ: {"eq", " ="}}
		ps.r.debugf("inserting %s\n    type: %s    %s  %d\n", p.name, names[kind][0], names[kind][1], v)
	}
	return p
}

// collectArgStatInfo is parser.c collect_arg_stat_info: options.xstat of a
// reference file named on the command line (state.curdepth is 0, so -H
// follows it), fatal on failure.
func (ps *parser) collectArgStatInfo() (string, unix.Stat_t, bool) {
	name, ok := ps.collectArg()
	if !ok {
		return "", unix.Stat_t{}, false
	}
	st, err := ps.r.xstatName(name, name, 0)
	if err != nil {
		ps.r.fatalTargetFileError(err, name)
	}
	return name, st, true
}

// fatalTargetFileError is util.c fatal_target_file_error.
func (r *Run) fatalTargetFileError(err error, name string) {
	r.fatalf(err, "%s", r.quote(name))
}

func parseInum(ps *parser, e *parserEntry) bool {
	p := ps.insertNum(e)
	if p == nil {
		return false
	}
	p.estSuccessRate = 1e-6
	p.needInum, p.needStat, p.needType = true, false, false
	return true
}

func parseLinks(ps *parser, e *parserEntry) bool {
	p := ps.insertNum(e)
	if p == nil {
		return false
	}
	switch p.args.num.val {
	case 1:
		p.estSuccessRate = 0.99
	case 2:
		p.estSuccessRate = 0.01
	default:
		p.estSuccessRate = 1e-3
	}
	return true
}

// parseUid and parseGid are parser.c parse_uid / parse_gid.
func parseUid(ps *parser, e *parserEntry) bool { return ps.insertIDNum(e) }
func parseGid(ps *parser, e *parserEntry) bool { return ps.insertIDNum(e) }

func (ps *parser) insertIDNum(e *parserEntry) bool {
	p := ps.insertNum(e)
	if p == nil {
		return false
	}
	p.estSuccessRate = 0.2
	if p.args.num.val < 100 {
		p.estSuccessRate = 0.99
	}
	return true
}

type sizeInfo struct {
	kind      compKind
	blockSize int64
	size      uint64
}

// parseSize is parser.c parse_size. GNU strips the unit suffix from argv in
// place, so -D tree shows "-size 1" for "-size 1k".
func parseSize(ps *parser, e *parserEntry) bool {
	r := ps.r
	if ps.i >= len(ps.argv) {
		return false
	}
	arg := ps.argv[ps.i]
	if arg == "" {
		r.fatalf(nil, "invalid null argument to -size")
	}
	num, suffix := arg[:len(arg)-1], arg[len(arg)-1]
	var bs int64
	switch suffix {
	case 'b':
		bs = 512
	case 'c':
		bs = 1
	case 'k':
		bs = 1024
	case 'M':
		bs = 1 << 20
	case 'G':
		bs = 1 << 30
	case 'w':
		bs = 2
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		num, suffix, bs = arg, 0, 512
	default:
		r.fatalf(nil, "invalid -size type `%s'", cbyte(suffix))
	}
	kind, rest := getCompType(num)
	v, ok := libc.Xstrtoumax(rest)
	if !ok {
		tail := ""
		if suffix != 0 {
			tail = cbyte(suffix)
		}
		r.fatalf(nil, "Invalid argument `%s%s' to -size", num, tail)
	}
	ps.argv[ps.i] = num // the in-place truncation, seen by build_expression_tree
	p := ps.insertPrimary(e, num, true)
	p.args.size = sizeInfo{kind, bs, v}
	p.needStat, p.needType = true, false
	big := v*uint64(bs) > 20480 // uintmax_t arithmetic: wraps like C
	switch {
	case kind == compGT && big, kind == compLT && !big:
		p.estSuccessRate = 0.1
	case kind == compGT, kind == compLT:
		p.estSuccessRate = 0.9
	default:
		p.estSuccessRate = 0.01
	}
	ps.i++
	return true
}

// parseSamefile is parser.c parse_samefile. O_NOFOLLOW is available, so
// the opened file's fstat wins whenever the open succeeds, and the
// descriptor stays open for the whole run (GNU's guard against inode reuse).
func parseSamefile(ps *parser, e *parserEntry) bool {
	r := ps.r
	name, st, ok := ps.collectArgStatInfo()
	if !ok {
		return false
	}
	flags := unix.O_RDONLY | unix.O_CLOEXEC
	if r.opt.Symlinks == SymlinkNeverDeref {
		flags |= unix.O_NOFOLLOW
	}
	if fd, err := unix.Open(name, flags, 0); err == nil {
		var fst unix.Stat_t
		if err := unix.Fstat(fd, &fst); err != nil {
			r.fatalTargetFileError(err, name)
		}
		if _, err := r.xstatName(name, name, 0); err != nil {
			r.fatalTargetFileError(err, name)
		}
		st = fst
		r.heldFDs = append(r.heldFDs, fd)
	}
	p := ps.insertPrimary(e, name, true)
	p.args.samefile = devIno{st.Dev, st.Ino}
	p.needType, p.needStat = false, true
	p.estSuccessRate = 0.01
	return true
}

func parseEmpty(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.estSuccessRate = 0.01
	return true
}

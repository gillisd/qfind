package find

import (
	"golang.org/x/sys/unix"

	"qfind/internal/modechange"
)

// permKind is enum permissions_type (defs.h).
type permKind uint8

const (
	permAtLeast permKind = iota // -MODE
	permAny                     // /MODE
	permExact                   // MODE
)

// permVal is struct perm_val (defs.h).
type permVal struct {
	kind permKind
	val  [2]uint32 // for a non-directory, for a directory
}

// parsePerm is parser.c parse_perm.
func parsePerm(ps *parser, e *parserEntry) bool {
	r := ps.r
	expr, ok := ps.collectArg()
	if !ok {
		return false
	}
	kind, rate, start := permExact, float32(0.01), 0
	if expr != "" {
		switch expr[0] {
		case '-':
			kind, rate, start = permAtLeast, 0.2, 1
		case '/':
			kind, rate, start = permAny, 0.3, 1
		}
	}
	mc, ok := modechange.Compile(expr[start:])
	// +NUMERICMODE was a GNU extension, withdrawn to agree with chmod.
	if !ok || (len(expr) > 1 && expr[0] == '+' && expr[1] >= '0' && expr[1] < '8') {
		r.fatalf(nil, "invalid file mode %s", r.quote(expr))
	}
	val := [2]uint32{mc.Adjust(0, false), mc.Adjust(0, true)}
	// An empty expr never compiles, so expr[0] exists here.
	if expr[0] == '/' && val[0] == 0 && val[1] == 0 {
		r.errorf(nil, "warning: you have specified a mode pattern %s (which is equivalent to /000). The meaning of -perm /000 was changed in 2007 to be consistent with -perm -000; that is, while it used to match no files, it now matches all files. This warning message will be removed in a future findutils release.", expr)
		kind, rate = permAtLeast, 0.9986
	}
	p := ps.insertPrimary(e, expr, true)
	p.estSuccessRate = rate
	p.args.perm = permVal{kind, val}
	return true
}

// predPerm is pred.c pred_perm.
func predPerm(r *Run, e *Entry, p *Predicate) bool {
	mode := e.st.Mode
	val := p.args.perm.val[0]
	if mode&unix.S_IFMT == unix.S_IFDIR {
		val = p.args.perm.val[1]
	}
	switch p.args.perm.kind {
	case permAtLeast:
		return mode&val == val
	case permAny:
		return val == 0 || mode&val != 0 // Savannah bug 14748: /000 matches all
	}
	return mode&07777 == val // MODE_ALL
}

// parseAccesscheck is parser.c parse_accesscheck.
func parseAccesscheck(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.needStat, p.needType = false, false
	p.sideEffects, p.noDefaultPrint = false, false
	p.estSuccessRate = 0.9
	if e.id == fnExecutable {
		p.estSuccessRate = 0.2
	}
	return true
}

// The access tests are pred.c's faccessat(cwd_dir_fd, rel_pathname, mode,
// 0): real IDs, following symbolic links. With no flags x/sys/unix makes
// the plain faccessat system call, as glibc does.
func predReadable(r *Run, e *Entry, p *Predicate) bool {
	return unix.Faccessat(unix.AT_FDCWD, e.sysPath, unix.R_OK, 0) == nil
}

func predWritable(r *Run, e *Entry, p *Predicate) bool {
	return unix.Faccessat(unix.AT_FDCWD, e.sysPath, unix.W_OK, 0) == nil
}

func predExecutable(r *Run, e *Entry, p *Predicate) bool {
	return unix.Faccessat(unix.AT_FDCWD, e.sysPath, unix.X_OK, 0) == nil
}

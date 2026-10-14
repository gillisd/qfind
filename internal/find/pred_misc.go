package find

import (
	"golang.org/x/sys/unix"

	"qfind/internal/fnmatch"
	"qfind/internal/mounts"
)

// parseFstype is parser.c parse_fstype. From -O2, a type that no mounted
// file system has can never match, so the test becomes -false (renamed
// back to -fstype by the parse loop, as in GNU).
func parseFstype(ps *parser, e *parserEntry) bool {
	r := ps.r
	name, ok := ps.collectArg()
	if !ok {
		return false
	}
	if r.opt.OptimisationLevel < 2 || mounts.HasType(name) {
		p := ps.insertPrimary(e, name, true)
		p.args.str = name
		p.estSuccessRate = r.estimateFstypeSuccessRate(name)
		return true
	}
	if r.opt.Debug&DebugTreeOpt != 0 {
		r.debugf("-fstype %s can never succeed, substituting -false\n", name)
	}
	return ps.insertFalse()
}

// estimateFstypeSuccessRate is parser.c estimate_fstype_success_rate.
func (r *Run) estimateFstypeSuccessRate(name string) float32 {
	var st unix.Stat_t
	if unix.Stat("/", &st) != nil {
		return 1.0
	}
	t, ok := mounts.FSType(st.Dev)
	if !ok {
		r.fatalf(nil, "Cannot read mounted file system list")
	}
	if t == name {
		return 0.7
	}
	return 0.3
}

// predFstype is pred.c pred_fstype.
func predFstype(r *Run, e *Entry, p *Predicate) bool { return r.fsType(e) == p.args.str }

// parseContext is parser.c parse_context: insert_primary(entry, NULL)
// with the pattern taken from argv directly; need_type stays set.
func parseContext(ps *parser, e *parserEntry) bool {
	r := ps.r
	if ps.i >= len(ps.argv) {
		return false
	}
	if !mounts.SELinuxEnabled() {
		r.fatalf(nil, "invalid predicate -context: SELinux is not enabled.")
	}
	p := ps.insertPrimary(e, "", false)
	p.estSuccessRate = 0.01
	p.needStat = false
	p.args.str = ps.argv[ps.i]
	ps.i++
	return true
}

// predContext is pred.c pred_context: fnmatch(pattern, context, 0); a
// getfilecon failure is diagnosed without changing the exit status.
func predContext(r *Run, e *Entry, p *Predicate) bool {
	ctx, err := r.getfilecon(e)
	if err != nil {
		r.errorAt(e, err, "getfilecon failed: %s", r.quote(e.path))
		return false
	}
	return fnmatch.Match(p.args.str, ctx, r.fnOpts(false))
}

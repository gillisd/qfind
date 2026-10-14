package find

import "golang.org/x/sys/unix"

// parseDelete is parser.c parse_delete: -delete implies -depth. It needs
// no stat: pred_delete retries an EISDIR unlink as rmdir.
func parseDelete(ps *parser, e *parserEntry) bool {
	p := ps.insertPrimaryNoArg(e)
	p.sideEffects, p.noDefaultPrint = true, true
	ps.r.opt.DoDirFirst = false
	p.needStat, p.needType = false, false
	p.estSuccessRate = 1.0
	return true
}

// predDelete is pred.c pred_delete with perform_delete's unlinkat.
func predDelete(r *Run, e *Entry, p *Predicate) bool {
	if e.accPath == "." {
		return true
	}
	flags := 0
	if e.haveStat && e.st.Mode&unix.S_IFMT == unix.S_IFDIR {
		flags = unix.AT_REMOVEDIR
	}
	err := unix.Unlinkat(unix.AT_FDCWD, e.sysPath, flags)
	if err == nil {
		return true
	}
	if err == unix.ENOENT && r.opt.IgnoreReaddirRace {
		return true
	}
	if err == unix.EISDIR && flags == 0 {
		if err = unix.Unlinkat(unix.AT_FDCWD, e.sysPath, unix.AT_REMOVEDIR); err == nil {
			return true
		}
	}
	r.errorAt(e, err, "cannot delete %s", r.quote(e.path))
	r.setFailure()
	return false
}

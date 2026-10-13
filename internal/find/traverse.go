package find

import "golang.org/x/sys/unix"

// processAllStartPoints is ftsfind.c process_all_startpoints (argv form;
// -files0-from arrives in Task 4). An empty name is reported and skipped,
// and makes the result false, so main skips the success rates and
// cleanup() as GNU does.
func (r *Run) processAllStartPoints() bool {
	if len(r.startPoints) == 0 {
		return r.findRoot(".")
	}
	ok := true
	for _, name := range r.startPoints {
		if name == "" {
			r.errorf(unix.ENOENT, "%s", r.quote(name))
			r.setFailure()
			ok = false
			continue
		}
		if !r.findRoot(name) {
			return false
		}
	}
	return ok
}

// findRoot evaluates a starting point at depth 0 (descent: Task 4). The
// stat is fts_open's fts_stat: -H and -L follow a link and fall back to
// the link itself only on ENOENT (FTS_SLNONE), never with debug_stat.
func (r *Run) findRoot(arg string) bool {
	e := &Entry{path: arg, sysPath: arg, name: arg, accPath: arg}
	var st unix.Stat_t
	var err error
	if r.opt.Symlinks == SymlinkNeverDeref {
		err = unix.Lstat(arg, &st)
	} else if err = unix.Stat(arg, &st); err == unix.ENOENT && unix.Lstat(arg, &st) == nil {
		err = nil
		// consider_visiting: FTS_SLNONE is checked for a link loop
		// through options.xstat.
		if _, lerr := r.xstat(e); lerr == unix.ELOOP {
			err = lerr
		}
	}
	if err != nil {
		r.nonfatalTargetFileError(e, err)
		return true
	}
	e.st, e.haveStat, e.haveType, e.mode = st, true, true, st.Mode
	if r.opt.MinDepth <= 0 {
		r.visit(e)
	}
	return true
}

// visit is ftsfind.c visit.
func (r *Run) visit(e *Entry) {
	r.apply(e, r.tree)
	r.commit(e)
}

// cleanup is util.c cleanup (Tasks 7–8 add exec batches and shared files).
func (r *Run) cleanup() {
	r.cleanupFlush()
}

package find

import "golang.org/x/sys/unix"

// predExec is exec.c impl_pred_exec for all four actions (pred_ok and
// pred_okdir prompt first). Evaluation is serialised whenever an
// exec-family action is present (needsSerial), so at most one predExec
// runs at a time.
func predExec(r *Run, e *Entry, p *Predicate) bool {
	ev := p.args.exec
	if ev.id == fnOk || ev.id == fnOkdir {
		if !r.askOk(e, ev) {
			return false
		}
	}
	target, prefix, dir := e.path, "", ""
	if ev.local {
		target = baseName(e.accPath) // base_name(state.rel_pathname)
		if target == "" || target[0] != '/' {
			prefix = "./"
		}
	}
	if ev.multiple {
		ev.mu.Lock()
		defer ev.mu.Unlock()
		s := &ev.batch
		if ev.local {
			s = ev.dirBatch(r, e)
		}
		ev.ctl.push(r, e, s, target, prefix, false, false, func() { r.runBatch(e, ev, s) })
		if ev.local && len(ev.dirBatches) > maxPendingDirBatches {
			r.completePendingDirBatches(ev) // bound memory; grouping is incidental
		}
		return true // POSIX: "+" is always true
	}
	if ev.local {
		dir = r.recordExecDir(e)
	}
	// bc_do_insert pushes each expanded word onto an empty command line.
	s := &bcState{}
	for _, t := range ev.replaceVec {
		ev.ctl.push(r, e, s, ev.ctl.insert(r, e, t, prefix, target), "", false, true, nil)
	}
	return r.launch(e, ev, s.argv, dir) == 0
}

// maxPendingDirBatches bounds the -execdir … + command lines kept open at
// once. GNU has one at a time because it flushes at every level change;
// keeping more lets one directory's names share a command line.
const maxPendingDirBatches = 1024

// dirBatch returns the -execdir + batch for e's directory and level. A new
// batch records its working directory (record_exec_dir runs only while
// state.todo is false) and starts from the initial arguments.
func (ev *execVal) dirBatch(r *Run, e *Entry) *bcState {
	key := batchKey{e.dirPath(), e.depth}
	s := ev.dirBatches[key]
	if s == nil {
		s = &bcState{argv: append([]string(nil), ev.batch.argv[:ev.ctl.initialArgc]...),
			chars: ev.batch.initialChars, initialChars: ev.batch.initialChars}
		ev.dirBatches[key] = s
	}
	if !s.todo {
		s.dir = r.recordExecDir(e)
	}
	return s
}

// completePendingDirBatches runs and forgets every pending -execdir batch
// of ev (do_complete_pending_execdirs for one predicate).
func (r *Run) completePendingDirBatches(ev *execVal) {
	for key, s := range ev.dirBatches {
		if s.todo {
			r.runBatch(nil, ev, s)
		}
		delete(ev.dirBatches, key)
	}
}

// runBatch is bc_do_exec + bc_clear_args with launch() as the callback.
// For the '+' form a non-zero child status (or a signal) makes find's
// exit status 1 (launch's execp->multiple branch).
func (r *Run) runBatch(e *Entry, ev *execVal, s *bcState) {
	if status := r.launch(e, ev, append([]string(nil), s.argv...), s.dir); status != 0 {
		r.setFailure()
	}
	s.reset(&ev.ctl)
}

// completePendingExecs is util.c complete_pending_execs over the tree, in
// left-to-right order. Like GNU's, it also runs pending -execdir … +
// batches, which reach it only at -quit or at the very end.
func (r *Run) completePendingExecs(p *Predicate) {
	if p == nil {
		return
	}
	r.completePendingExecs(p.left)
	if ev := p.args.exec; ev != nil && ev.multiple {
		if ev.local {
			r.completePendingDirBatches(ev)
		} else if ev.batch.todo {
			r.runBatch(nil, ev, &ev.batch)
		}
	}
	r.completePendingExecs(p.right)
}

// completePendingExecdirs is util.c complete_pending_execdirs.
func (r *Run) completePendingExecdirs(p *Predicate) {
	if p == nil {
		return
	}
	r.completePendingExecdirs(p.left)
	if ev := p.args.exec; ev != nil && ev.multiple && ev.local {
		r.completePendingDirBatches(ev)
	}
	r.completePendingExecdirs(p.right)
}

// recordExecDir is exec.c record_exec_dir + initialize_wd_for_exec. GNU
// opens the working directory O_RDONLY relative to fts's cwd: "." inside
// the parent for an entry below a start point when fts changed directory
// (needs read and search permission on the parent), mdir_name(rel_pathname)
// otherwise (the start point, or every entry under -L's FTS_NOCHDIR).
// Failure is fatal; GNU's message quotes rel_pathname. The descriptor is
// not kept: the child changes directory by name (launch).
func (r *Run) recordExecDir(e *Entry) string {
	dir := e.dirPath()
	probe := dir
	if e.depth > 0 && r.opt.Symlinks != SymlinkAlwaysDeref {
		probe = dir + "/."
	}
	fd, err := unix.Open(probe, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		r.fatalAt(e, err, "Failed to save working directory in order to run a command on %s", r.quote(e.accPath))
	}
	unix.Close(fd)
	return dir
}

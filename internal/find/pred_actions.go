package find

// predPrint is pred.c pred_print.
func predPrint(r *Run, e *Entry, p *Predicate) bool {
	r.emitQuoted(e, p.args.out, e.path, "\n")
	return true
}

// predPrint0 is pred.c pred_print0 → pred_fprint0: never quoted.
func predPrint0(r *Run, e *Entry, p *Predicate) bool {
	r.emit(e, p.args.out, e.path+"\x00")
	return true
}

// predPrune is pred.c pred_prune.
func predPrune(r *Run, e *Entry, p *Predicate) bool {
	if r.opt.DoDirFirst && e.haveStat && e.st.Mode&unixS_IFMT == unixS_IFDIR {
		e.prune = true
	}
	return true
}

// predQuit is pred.c pred_quit: cleanup, then exit with the current status.
// Evaluation is serialised when -quit is present (Task 4), so nothing else
// can print after this point.
func predQuit(r *Run, e *Entry, p *Predicate) bool {
	r.commit(e)
	r.endHold(e) // cleanup locks every sink
	r.cleanup()
	r.exit(int(r.exitStatus.Load()))
	return true
}

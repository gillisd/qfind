package find

import "slices"

// recordState is one -printf/-fprintf evaluation in progress on an entry,
// and the sinks a diagnostic made the entry hold. GNU writes a format's
// segments one stdio call at a time, and glibc's error() flushes stdout
// between two of them, so a diagnostic raised mid-record (%l, %Y, %Z, an
// EOVERFLOW) must send the record's earlier bytes to their sinks first.
// Committing would release the sinks and let another entry's output land
// inside the record, which single-threaded GNU never does; so the entry
// keeps every sink locked from its first diagnostic until its visit ends
// (visit, endHold): a later diagnostic of the same entry, in another
// record or action, then follows the entry's own output in a merged
// stream, as it does in GNU.
type recordState struct {
	open     bool
	out      sink   // the format's stream (nil for stderr)
	held     []sink // locked by errorInRecord, in lock order
	visiting bool   // inside visit: a diagnostic's hold lasts until endHold
}

// holds reports whether e's open record holds sk's lock.
func (e *Entry) holds(sk sink) bool {
	return e != nil && slices.Contains(e.record.held, sk)
}

// openRecord starts pred_fprintf's record on out.
func (r *Run) openRecord(e *Entry, out *Stream) {
	e.record.open, e.record.out = true, out.sink
}

// closeRecord ends the record: if a diagnostic made it hold its sinks,
// the rest of the record goes out under them. Inside a visit they stay
// held until endHold; the window is the rest of one entry's evaluation.
func (r *Run) closeRecord(e *Entry) {
	e.record.open, e.record.out = false, nil
	if len(e.record.held) > 0 {
		r.commit(e)
		if !e.record.visiting {
			r.releaseRecord(e)
		}
	}
}

// endHold commits e's output and releases every sink it holds: at the
// end of its visit, and before anything that takes a lock ordered before
// the sinks (-exec's execVal.mu and execMu) or exits (-quit's cleanup).
func (r *Run) endHold(e *Entry) {
	if len(e.record.held) > 0 {
		r.commit(e)
		r.releaseRecord(e)
	}
}

// releaseRecord unlocks every sink the record holds (also before any
// fatal exit: exit() needs them).
func (r *Run) releaseRecord(e *Entry) {
	if e == nil {
		return
	}
	for i := len(e.record.held) - 1; i >= 0; i-- {
		e.record.held[i].unlock()
	}
	e.record.held = nil
}

// errorInRecord is errorAt inside an open record: lock the sinks of e's
// pending calls and the record's own stream (keeping those already held),
// commit under those locks, then do error()'s stdout flush and print the
// message without taking a held lock again. A second diagnostic in the
// same record finds every sink it needs already held: after the first,
// only the record's own stream receives calls.
func (r *Run) errorInRecord(e *Entry, err error, format string, a ...any) {
	r.holdRecordSinks(e)
	r.commit(e)
	o, msg := r.out, r.errorText(err, format, a...)
	if e.holds(o) {
		o.f.flushBuf()
		r.writeStderr(msg)
	} else {
		o.diagnose(msg)
	}
}

// holdRecordSinks locks, in the global order (shared files by opening
// order, then stdout), every sink e's record needs and does not hold yet.
// Sinks are only ever added while none of the new ones precedes a held
// one: the first call takes them all, later calls need nothing new. A
// hold that outlives its record (visiting) may meet any of the
// expression's streams later, so it takes every shared file and stdout.
func (r *Run) holdRecordSinks(e *Entry) {
	var need []sink
	add := func(sk sink) {
		if sk != nil && !e.holds(sk) && !slices.Contains(need, sk) {
			need = append(need, sk)
		}
	}
	for _, it := range e.txn.items {
		if !it.done {
			add(it.s.sink)
		}
	}
	add(e.record.out)
	if e.record.visiting && len(e.record.held) == 0 {
		for _, sf := range r.shared.all() {
			add(sf)
		}
		add(r.out)
	}
	slices.SortFunc(need, func(a, b sink) int { return sinkRank(a) - sinkRank(b) })
	for _, sk := range need {
		sk.lock()
		e.record.held = append(e.record.held, sk)
	}
}

// sinkRank orders sinks for locking: shared files before stdout.
func sinkRank(sk sink) int {
	if sf, ok := sk.(*sharedFile); ok {
		return sf.seq
	}
	return 1 << 30
}

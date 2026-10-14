package find

import (
	"hash/maphash"
	"sync"
)

// stdoutBatch gathers the plain stdout records (-print, -print0) of
// non-directory entries in parallel mode, so that walker goroutines take
// stdout's lock once per batch instead of once per file: per-entry
// locking was a quarter of a -print walk's CPU time, all of it
// contention. A record still reaches stdout whole, and only order
// between unrelated entries changes, which GNU does not guarantee:
//   - a directory's records are committed directly, before fastwalk can
//     read the directory, so they still precede its contents';
//   - every slot is drained when a walk ends, before -depth's deferred
//     directory visits and before the next start point (drain), and
//     before exit flushes stdout (exitClaimed);
//   - checked or rendered calls (-printf, -ls), other sinks, and entries
//     that diagnosed something are committed directly, as is everything
//     in serial mode (-exec & co., -quit, -D) or on a line-buffered
//     terminal.
//
// Its slot locks precede stdout's lock in the global order.
type stdoutBatch struct {
	seed  maphash.Seed
	slots []batchSlot
}

type batchSlot struct {
	mu  sync.Mutex
	buf []byte
	_   [40]byte // keep slots on separate cache lines
}

// batchFlushSize is how much a slot gathers before it writes to stdout.
const batchFlushSize = 16 << 10

func newStdoutBatch(workers int) *stdoutBatch {
	return &stdoutBatch{seed: maphash.MakeSeed(), slots: make([]batchSlot, 4*max(1, workers))}
}

// batchable reports whether e's pending output is only unchecked stdout
// calls, which may wait in a slot.
func (r *Run) batchable(e *Entry) bool {
	if r.batch == nil || e.isDir() || len(e.record.held) > 0 || len(e.txn.items) == 0 {
		return false
	}
	for i := range e.txn.items {
		it := &e.txn.items[i]
		if it.done || it.s.sink != sink(r.out) || it.render != nil || it.fail != failIgnored {
			return false
		}
	}
	return true
}

// add moves e's pending stdout calls into a slot. Entries of one
// directory, which one fastwalk goroutine reads in sequence, hash to the
// same first slot; a busy slot is skipped rather than waited for.
func (r *Run) batchAdd(e *Entry) {
	b := r.batch
	n := len(b.slots)
	i := int(maphash.String(b.seed, e.sysPath[:len(e.sysPath)-len(e.name)]) % uint64(n))
	var s *batchSlot
	for k := 0; k < n; k++ {
		if c := &b.slots[(i+k)%n]; c.mu.TryLock() {
			s = c
			break
		}
	}
	if s == nil {
		s = &b.slots[i]
		s.mu.Lock()
	}
	for _, it := range e.txn.items {
		s.buf = append(append(s.buf, it.text...), it.suffix...)
	}
	e.txn.items = e.txn.items[:0]
	if len(s.buf) >= batchFlushSize {
		r.flushSlot(s)
	}
	s.mu.Unlock()
}

// flushSlot writes a slot's records to stdout as one unchecked call; the
// caller holds the slot's lock.
func (r *Run) flushSlot(s *batchSlot) {
	if len(s.buf) == 0 {
		return
	}
	o := r.out
	o.lock()
	o.write(string(s.buf), "")
	o.unlock()
	s.buf = s.buf[:0]
}

// drainBatch writes every slot to stdout.
func (r *Run) drainBatch() {
	if r.batch == nil {
		return
	}
	for i := range r.batch.slots {
		s := &r.batch.slots[i]
		s.mu.Lock()
		r.flushSlot(s)
		s.mu.Unlock()
	}
}

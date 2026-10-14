package find

import (
	"slices"
	"sync"
)

// deferredDirs holds directories whose FTS_DP visit must wait until their
// subtree has been walked. Files are never deferred: for them pre-order
// and post-order coincide.
type deferredDirs struct {
	mu      sync.Mutex
	byDepth map[int][]*Entry
}

func (d *deferredDirs) add(e *Entry) {
	d.mu.Lock()
	if d.byDepth == nil {
		d.byDepth = map[int][]*Entry{}
	}
	d.byDepth[e.depth] = append(d.byDepth[e.depth], e)
	d.mu.Unlock()
}

// take empties the set and returns its levels, deepest first.
func (d *deferredDirs) take() [][]*Entry {
	d.mu.Lock()
	defer d.mu.Unlock()
	depths := make([]int, 0, len(d.byDepth))
	for depth := range d.byDepth {
		depths = append(depths, depth)
	}
	slices.Sort(depths)
	slices.Reverse(depths)
	levels := make([][]*Entry, len(depths))
	for i, depth := range depths {
		levels[i] = d.byDepth[depth]
	}
	d.byDepth = nil
	return levels
}

// runDeferred visits deferred directories deepest level first, after the
// walk of the starting point has finished (fastwalk has no "directory
// done" event). Every descendant of a directory is either a file, visited
// during the walk, or a directory at a deeper level, visited earlier here;
// so this is a valid post-order. Directories at one level are never
// ancestors of one another, so a level is evaluated in parallel, unless
// evaluation is serialised anyway.
func (w *walker) runDeferred() {
	for _, level := range w.defers.take() {
		if w.r.serial || len(level) == 1 {
			for _, e := range level {
				w.visitPostOrder(e)
			}
			continue
		}
		sem := make(chan struct{}, w.r.workers)
		var wg sync.WaitGroup
		for _, e := range level {
			sem <- struct{}{}
			wg.Go(func() {
				defer func() { <-sem }()
				w.visitPostOrder(e)
			})
		}
		wg.Wait()
	}
}

// visitPostOrder is consider_visiting for a directory's FTS_DP entry (or
// FTS_DNR, see considerVisiting) under -depth: narrate, then visit with
// the stat taken in pre-order, as fts does.
func (w *walker) visitPostOrder(e *Entry) {
	r := w.r
	if r.serial {
		r.evalMu.Lock()
		defer r.evalMu.Unlock()
	}
	if e.cancelled.Load() {
		return
	}
	w.narrateEarly(e, ftsDP)
	if r.opt.Debug&DebugSearch != 0 {
		r.debugf("consider_visiting (late): %s: fts_info=%-6s, isdir=%d ignore=%d have_stat=%d have_type=%d \n",
			r.quote(e.path), ftsInfoNames[ftsDP], 1, 0, b2i(e.haveStat), b2i(e.haveType))
	}
	r.visit(e)
}

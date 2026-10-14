package find

import (
	"sync"
	"sync/atomic"
)

// dirNode is one directory on the active path: (dev, ino) plus its parent.
// fastwalk gives no "directory finished" event, so nodes live until the
// start point is done (memory O(directories), never O(files)).
type dirNode struct {
	dev, ino uint64
	parent   *dirNode
	entry    *Entry       // the directory itself: read errors report it, -depth visits it
	nitems   atomic.Int64 // children fastwalk has delivered: fts_build's nitems
}

type dirTree struct{ m sync.Map } // fastwalk dir path → *dirNode

func (t *dirTree) add(key string, n *dirNode) { t.m.Store(key, n) }

func (t *dirTree) get(key string) *dirNode {
	if v, ok := t.m.Load(key); ok {
		return v.(*dirNode)
	}
	return nil
}

// isCycle reports whether (dev, ino) is an active ancestor (fts FTS_DC):
// fts-cycle.c enter_dir with FTS_TIGHT_CYCLE_CHECK keeps exactly the
// directories from the root down to the one being read in its hash table.
// Only ancestors count: noleaf_spec requires a directory reached through
// two non-ancestor paths to be walked twice.
func isCycle(parent *dirNode, dev, ino uint64) bool {
	for n := parent; n != nil; n = n.parent {
		if n.dev == dev && n.ino == ino {
			return true
		}
	}
	return false
}

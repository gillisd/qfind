package find

import (
	"io/fs"
	"strconv"
	"sync"

	"golang.org/x/sys/unix"
)

// deepThreshold is the fastwalk path length above which a directory is
// re-rooted: its entries' paths, at most NAME_MAX+1 bytes longer, must
// stay below PATH_MAX for fastwalk's open(2) and qfind's lstat(2). GNU's
// fts (FTS_CWDFD) walks any depth through directory descriptors; qfind
// gets the same reach by walking such a directory again with fastwalk,
// rooted at /proc/self/fd/N. -L is the exception: FTS_LOGICAL implies
// FTS_NOCHDIR, so GNU addresses every file by its full path and fails
// with ENAMETOOLONG exactly where fastwalk does (FTS_NS for the first
// entry too long to stat, which is still visited).
const deepThreshold = unix.PathMax - 255 - 2

// reroots reports whether a directory whose fastwalk path is key must be
// walked from /proc/self/fd/N instead.
func (r *Run) reroots(key string) bool {
	return len(key) > deepThreshold && r.opt.Symlinks != SymlinkAlwaysDeref
}

// deepDir is a directory to walk from /proc/self/fd/<fd>, once opened.
type deepDir struct {
	e      *Entry
	node   *dirNode
	prefix string // fts child prefix: the directory's fts_path plus "/"
}

// deepQueue collects one walk's deep directories; that walk's callbacks
// append to it concurrently.
type deepQueue struct {
	mu   sync.Mutex
	todo []deepDir
}

// deepen is called by considerDir instead of letting fastwalk descend
// into a directory whose path is too long. The directory has already
// been considered (visited, or deferred under -depth); only its walk is
// postponed. Nothing is opened yet: descriptors are opened one subtree at
// a time (walkDeepDir), so their number does not grow with fan-out.
func (w *walker) deepen(e *Entry, node *dirNode) {
	w.deep.mu.Lock()
	w.deep.todo = append(w.deep.todo, deepDir{e, node, e.path + "/"})
	w.deep.mu.Unlock()
}

// walkDeep walks the directories w's finished walk postponed, one after
// another. Their sysPaths may live below w's own /proc/self/fd/N, which
// stays open until this returns.
func (w *walker) walkDeep() {
	w.deep.mu.Lock()
	todo := w.deep.todo
	w.deep.todo = nil
	w.deep.mu.Unlock()
	for _, d := range todo {
		w.walkDeepDir(d)
	}
}

// walkDeepDir opens d (failure is fts's FTS_DNR) and finishes its whole
// subtree before closing the descriptor: the walk, the deep directories
// it postpones in turn, the deferred -depth visits below d (all of them
// descendants of d, so visiting them now is a valid post-order; d itself
// is deferred in its parent's walk), and the pending -execdir … + command
// lines, whose working directories may be /proc/self/fd/N paths. At any
// time only the chain of re-rooted ancestors holds descriptors, one per
// ~3,800 bytes of path. O_CLOEXEC keeps them from every child.
func (w *walker) walkDeepDir(d deepDir) {
	fd, err := unix.Open(d.e.sysPath, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		w.readError(d.e, false, &fs.PathError{Op: "open", Path: d.e.sysPath, Err: err})
		return
	}
	fw := "/proc/self/fd/" + strconv.Itoa(fd)
	nw := *w
	nw.rp = rootPaths{arg: d.e.path, fw: fw, fwSep: fw + "/", prefix: d.prefix}
	nw.root, nw.baseDepth = d.e, d.e.depth
	nw.dirs, nw.defers, nw.deep = &dirTree{}, &deferredDirs{}, &deepQueue{} // keys and fds are this walk's own
	nw.dirs.add(fw, d.node)
	nw.walk()
	nw.walkDeep()
	nw.runDeferred()
	w.r.completePendingExecdirs(w.r.tree)
	unix.Close(fd)
}

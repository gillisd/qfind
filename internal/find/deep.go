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

// deepDir is a directory to walk from /proc/self/fd/<fd>.
type deepDir struct {
	e    *Entry
	node *dirNode
	fd   int
}

// deepQueue is shared by every walker of one start point.
type deepQueue struct {
	mu   sync.Mutex
	todo []deepDir
	fds  []int // kept open until the start point is finished
}

// deepen is called by considerDir instead of letting fastwalk descend
// into a directory whose path is too long. The directory has already
// been considered (visited, or deferred under -depth); only its walk is
// postponed. It reports false when the directory cannot be opened, which
// is fts's FTS_DNR. The descriptor is O_CLOEXEC, so no child inherits it.
func (w *walker) deepen(e *Entry, key string, node *dirNode) bool {
	fd, err := unix.Open(e.sysPath, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		w.readError(e, &fs.PathError{Op: "open", Path: key, Err: err})
		return false
	}
	w.deep.mu.Lock()
	w.deep.todo = append(w.deep.todo, deepDir{e, node, fd})
	w.deep.fds = append(w.deep.fds, fd)
	w.deep.mu.Unlock()
	return true
}

// walkDeep walks every postponed directory, and the ones those walks
// postpone in turn. Each walk shares the start point's dirTree (cycle
// detection) and deferred list (-depth).
func (w *walker) walkDeep() {
	for {
		w.deep.mu.Lock()
		if len(w.deep.todo) == 0 {
			w.deep.mu.Unlock()
			return
		}
		d := w.deep.todo[0]
		w.deep.todo = w.deep.todo[1:]
		w.deep.mu.Unlock()
		fw := "/proc/self/fd/" + strconv.Itoa(d.fd)
		w.dirs.add(fw, d.node)
		nw := *w
		nw.rp = rootPaths{arg: d.e.path, fw: fw, fwSep: fw + "/", prefix: d.e.path + "/"}
		nw.root, nw.baseDepth = d.e, d.e.depth
		nw.walk()
	}
}

// closeDeep closes the re-rooting descriptors once nothing can use a
// sysPath below them: after the deferred visits and pending -execdir runs.
func (w *walker) closeDeep() {
	for _, fd := range w.deep.fds {
		unix.Close(fd)
	}
	w.deep.fds = nil
}

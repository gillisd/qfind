package find

import (
	"github.com/charlievieth/fastwalk"
	"golang.org/x/sys/unix"
)

// walker is the state of one fastwalk.Walk below a start point, or below
// a very deep directory re-rooted at /proc/self/fd/N (deep.go).
type walker struct {
	r         *Run
	rp        rootPaths
	root      *Entry // the directory this Walk starts at
	baseDepth int    // fts_level of root: 0 for a start point
	rootDev   uint64 // the start point's device (-xdev, -mount)
	dirs      *dirTree
	defers    *deferredDirs
	deep      *deepQueue // directories to re-root at /proc/self/fd/N (deep.go)
}

// findRoot is ftsfind.c find(): classify and visit the start point, then
// let fastwalk walk below it. A start point that is not a directory to
// descend never reaches fastwalk (-files0-from may supply 100,000 of them).
func (r *Run) findRoot(arg string) bool {
	w := &walker{r: r, rp: newRootPaths(arg), dirs: &dirTree{}, defers: &deferredDirs{}, deep: &deepQueue{}}
	root := &Entry{path: arg, sysPath: arg, name: arg, accPath: arg, startLen: len(arg)}
	w.root = root
	info, err := r.rootInfo(root)
	if w.considerRoot(root, info, err) {
		node := &dirNode{dev: root.st.Dev, ino: root.st.Ino, entry: root}
		w.rootDev = root.st.Dev
		if r.reroots(w.rp.fw) { // a start point too long for its children's paths
			w.walkDeepDir(deepDir{root, node, w.rp.prefix})
		} else {
			w.dirs.add(w.rp.fw, node)
			w.walk()
			w.walkDeep()
		}
	}
	w.runDeferred()
	r.afterRoot()
	return true
}

// walk runs fastwalk over w.rp's root. The callbacks never return an error
// that aborts the walk, so an error here is Walk's own os.Stat of the
// root, which lost a race with a rename: report it as a read error.
func (w *walker) walk() {
	conf := &fastwalk.Config{NumWorkers: w.r.workers, Sort: fastwalk.SortNone}
	if w.r.opt.MaxDepth > 0 {
		conf.MaxDepth = w.r.opt.MaxDepth - w.baseDepth // fastwalk counts from its own root
	}
	if err := fastwalk.Walk(conf, w.rp.fw, w.callback); err != nil {
		w.readError(w.root, false, err)
	}
}

// rootInfo is fts_open/fts_stat for a level-0 entry: lstat under -P;
// stat with FTS_COMFOLLOW under -H/-L, where only ENOENT falls back to
// lstat, as FTS_SLNONE (fts_stat, not util.c fallback_stat: ENOTDIR,
// EACCES and ELOOP are FTS_NS). fts's stats never print debug_stat.
func (r *Run) rootInfo(e *Entry) (ftsInfo, error) {
	var st unix.Stat_t
	if r.opt.Symlinks == SymlinkNeverDeref {
		if err := unix.Lstat(e.sysPath, &st); err != nil {
			return ftsNS, err
		}
	} else if err := unix.Stat(e.sysPath, &st); err != nil {
		if err != unix.ENOENT || unix.Lstat(e.sysPath, &st) != nil {
			return ftsNS, err
		}
		e.setStat(&st)
		return ftsSLNONE, nil
	}
	e.setStat(&st)
	return statInfo(st.Mode), nil
}

// afterRoot runs the pending -execdir … + command lines, so that one
// never spans starting points (GNU runs them whenever fts changes level).
func (r *Run) afterRoot() { r.completePendingExecdirs(r.tree) }

// visit is ftsfind.c visit.
func (r *Run) visit(e *Entry) {
	r.apply(e, r.tree)
	r.commit(e)
}

// cleanup is util.c cleanup: pending command lines first, then the
// shared output files, then fflush(stdout).
func (r *Run) cleanup() {
	r.completePendingExecs(r.tree)
	r.completePendingExecdirs(r.tree)
	r.closeSharedFiles()
	r.cleanupFlush()
}

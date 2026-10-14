package find

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/charlievieth/fastwalk"
	"golang.org/x/sys/unix"

	"qfind/internal/libc"
)

// callback is the fs.WalkDirFunc handed to fastwalk.Walk. It runs on
// several goroutines at once. fastwalk's contract shapes every return:
//   - a directory callback runs when the directory is dequeued, before it
//     is read; it returns nil (read it) or SkipDir, never another error;
//   - a non-directory callback returns nil, or ErrTraverseLink for a
//     followed symlink to a directory to descend; anything else would
//     abandon the parent directory;
//   - a second callback with err != nil reports that a directory could not
//     be read; it returns nil, because any error aborts the whole walk.
func (w *walker) callback(path string, d fs.DirEntry, err error) error {
	de := d.(fastwalk.DirEntry)
	depth := de.Depth()
	if err != nil {
		n := w.dirNodeAt(path, depth)
		w.readError(n.entry, n.nitems.Load() > 0, err)
		return nil
	}
	if depth == 0 {
		return nil // the root was considered before Walk
	}
	r := w.r
	gpath := w.rp.gnu(path)
	acc := de.Name()
	if r.opt.Symlinks == SymlinkAlwaysDeref {
		acc = gpath // FTS_LOGICAL implies FTS_NOCHDIR: fts_accpath is fts_path
	}
	e := newEntry()
	e.path, e.sysPath, e.name, e.accPath = gpath, path, de.Name(), acc
	e.depth, e.startLen = w.baseDepth+depth, w.root.startLen
	parent := w.dirs.get(w.rp.parentKey(path, depth))
	if parent != nil {
		parent.nitems.Add(1)
	}
	typ := de.Type()
	switch {
	case typ.IsDir():
		return w.considerDir(e, path, parent, false)
	case typ&fs.ModeSymlink != 0 && r.opt.Symlinks == SymlinkAlwaysDeref:
		return w.considerFollowedLink(e, path, parent)
	case r.opt.Mount:
		return w.considerMounted(e)
	}
	e.setType(typ)
	w.consider(e, ftsNSOK, nil)
	freeEntry(e) // a non-directory is never deferred or kept once considered
	return nil
}

// dirNodeAt returns the node of a directory fastwalk has read or tried to.
func (w *walker) dirNodeAt(path string, depth int) *dirNode {
	if depth == 0 {
		return w.dirs.get(w.rp.fw)
	}
	return w.dirs.get(path)
}

// consider is consider_visiting, under the evaluation lock when
// evaluation is serialised: GNU considers one entry at a time, so with
// -quit, -exec & co. or -D, no diagnostic or visit of another entry may
// interleave with this one.
func (w *walker) consider(e *Entry, info ftsInfo, err error) bool {
	if w.r.serial {
		w.r.evalMu.Lock()
		defer w.r.evalMu.Unlock()
	}
	return w.considerVisiting(e, info, err)
}

// considerDir handles an entry fts stats itself: a directory (or, under
// -L, a symlink to one). Its stat decides everything: the entry may have
// stopped being a directory since readdir, and fastwalk opens what it is
// told to read without O_DIRECTORY or O_NONBLOCK (a FIFO would block a
// worker), so only a confirmed directory is ever descended into.
func (w *walker) considerDir(e *Entry, key string, parent *dirNode, viaLink bool) error {
	r := w.r
	var st unix.Stat_t
	statFn := unix.Lstat
	if r.opt.Symlinks == SymlinkAlwaysDeref {
		statFn = unix.Stat
	}
	if err := statFn(e.sysPath, &st); err != nil {
		w.consider(e, ftsNS, err)
		return skip(viaLink)
	}
	e.setStat(&st)
	if r.opt.Mount && st.Dev != w.rootDev {
		return skip(viaLink) // FTS_MOUNT: never returned
	}
	if info := statInfo(st.Mode); info != ftsD {
		w.consider(e, info, nil)
		return skip(viaLink)
	}
	if isCycle(parent, st.Dev, st.Ino) {
		w.consider(e, ftsDC, nil)
		return skip(viaLink)
	}
	w.dirs.add(key, &dirNode{dev: st.Dev, ino: st.Ino, parent: parent, entry: e})
	descend := w.consider(e, ftsD, nil)
	if r.opt.Xdev && st.Dev != w.rootDev {
		descend = false // FTS_XDEV: visited, never entered
	}
	if !descend {
		return skip(viaLink)
	}
	if r.reroots(key) {
		w.deepen(e, w.dirs.get(key))
		return skip(viaLink)
	}
	if viaLink {
		return fastwalk.ErrTraverseLink
	}
	return nil
}

func skip(viaLink bool) error {
	if viaLink {
		return nil // fastwalk does not traverse a link unless asked
	}
	return fastwalk.SkipDir
}

// considerFollowedLink handles a symlink under -L, which fts stats
// (fts_stat with follow): ENOENT means a dangling link (FTS_SLNONE), other
// errors are FTS_NS, and a directory target is considered as a directory.
func (w *walker) considerFollowedLink(e *Entry, key string, parent *dirNode) error {
	r := w.r
	var st unix.Stat_t
	err := unix.Stat(e.sysPath, &st)
	switch {
	case err == nil && st.Mode&unix.S_IFMT == unix.S_IFDIR:
		return w.considerDir(e, key, parent, true)
	case err == nil:
		e.setStat(&st)
		if r.opt.Mount && st.Dev != w.rootDev {
			return nil
		}
		w.consider(e, statInfo(st.Mode), nil)
	case err == unix.ENOENT && unix.Lstat(e.sysPath, &st) == nil:
		e.setStat(&st)
		w.consider(e, ftsSLNONE, nil)
	default:
		w.consider(e, ftsNS, err)
	}
	return nil
}

// considerMounted is FTS_MOUNT's stat of every entry: entries on another
// device are never returned; a failed stat is (FTS_NS).
func (w *walker) considerMounted(e *Entry) error {
	var st unix.Stat_t
	if err := unix.Lstat(e.sysPath, &st); err != nil {
		w.consider(e, ftsNS, err)
		return nil
	}
	e.setStat(&st)
	if st.Dev != w.rootDev {
		return nil
	}
	w.consider(e, statInfo(st.Mode), nil)
	return nil
}

// readError maps fastwalk's report that it could not read a directory to
// the fts state GNU would see:
//   - open(2) failed: FTS_DNR;
//   - getdents(2) failed: FTS_ERR (fts reports it in place of FTS_DP),
//     but FTS_DNR while no entry has been read yet (fts_build's nitems):
//     a directory removed while open fails its first getdents with
//     ENOENT on some file systems, which -ignore_readdir_race ignores;
//   - lstat of a DT_UNKNOWN child failed: FTS_NS for that child. fastwalk
//     abandons the rest of the directory here, an accepted divergence.
func (w *walker) readError(dir *Entry, readSome bool, err error) {
	var pe *fs.PathError
	var se *os.SyscallError
	errno, ok := libc.Errno(err)
	if !ok {
		errno = unix.EIO
	}
	switch {
	case errors.As(err, &pe) && pe.Op == "lstat":
		// fastwalk lstats dirName + "/" + name, so a child of "/" is "//name".
		i := strings.LastIndexByte(pe.Path, '/')
		name, fwPath := pe.Path[i+1:], pe.Path
		if i > 0 && pe.Path[i-1] == '/' {
			fwPath = pe.Path[:i] + name
		}
		child := &Entry{path: w.rp.gnu(fwPath), sysPath: fwPath, name: name, accPath: name,
			depth: dir.depth + 1, startLen: w.root.startLen}
		if w.r.opt.Symlinks == SymlinkAlwaysDeref {
			child.accPath = child.path
		}
		w.consider(child, ftsNS, errno)
	case errors.As(err, &se) && readSome:
		w.consider(dir, ftsERR, errno)
	default:
		w.consider(dir, ftsDNR, errno)
	}
}

// considerVisiting is ftsfind.c consider_visiting for one entry; call it
// through consider. It returns whether a directory may be descended into.
func (w *walker) considerVisiting(e *Entry, info ftsInfo, err error) bool {
	r := w.r
	w.narrateEarly(e, info)
	switch info {
	case ftsERR:
		e.cancelled.Store(true) // fts returns FTS_ERR in place of FTS_DP
		e.issuedStatErr = false // a new fts_read iteration
		r.nonfatalTargetFileError(e, err)
		return false
	case ftsDNR:
		if errno, _ := libc.Errno(err); errno == unix.ENOENT && r.opt.IgnoreReaddirRace {
			e.cancelled.Store(true) // GNU returns before the post-order visit
			return false
		}
		e.issuedStatErr = false
		r.nonfatalTargetFileError(e, err)
		// Without -depth the directory was visited in pre-order. With it, the
		// deferred visit of e is GNU's visit of this FTS_DNR entry.
		return false
	case ftsDC:
		r.errorAt(e, nil, "File system loop detected; the following directory is part of the cycle: %s", r.quote(e.path))
		r.setFailure()
		return false
	case ftsSLNONE:
		if w.symlinkLoop(e) {
			r.nonfatalTargetFileError(e, unix.ELOOP)
			return false
		}
	case ftsNS:
		if e.depth == 0 {
			r.nonfatalTargetFileError(e, err)
			return false
		}
		if w.symlinkLoop(e) {
			r.nonfatalTargetFileError(e, unix.ELOOP)
			return false
		}
		if errno, _ := libc.Errno(err); errno == unix.ENOENT && r.opt.IgnoreReaddirRace {
			return false
		}
		r.nonfatalTargetFileError(e, err)
		// Visit without stat or type: fts zeroes the stat buffer, so a d_type
		// must not leak through (-type d would wrongly match).
		e.haveStat, e.haveType, e.mode = false, false, 0
	}
	if e.haveStat && e.st.Mode == 0 {
		r.errorAt(e, nil, "WARNING: file %s appears to have mode 0000", r.quote(e.path))
	}
	isDir := e.isDir() || info == ftsD
	descend := isDir
	ignore := false
	if r.opt.MaxDepth >= 0 && e.depth >= r.opt.MaxDepth {
		descend = false
		ignore = e.depth > r.opt.MaxDepth
	}
	postOrder := info == ftsD && !r.opt.DoDirFirst
	if e.depth < r.opt.MinDepth {
		ignore = true
	}
	if r.opt.Debug&DebugSearch != 0 {
		r.debugf("consider_visiting (late): %s: fts_info=%-6s, isdir=%d ignore=%d have_stat=%d have_type=%d \n",
			r.quote(e.path), ftsInfoNames[info], b2i(isDir), b2i(ignore || postOrder), b2i(e.haveStat), b2i(e.haveType))
	}
	switch {
	case ignore:
	case postOrder:
		w.defers.add(e) // visited as FTS_DP after the walk (depth.go)
	default:
		r.visit(e)
		if e.prune {
			descend = false
		}
	}
	return descend
}

// narrateEarly is consider_visiting's first -D search line. prev_depth is
// GNU's static: the level of the previously considered entry. It feeds
// only this narration, so it is tracked only under -D search (which
// serialises evaluation).
func (w *walker) narrateEarly(e *Entry, info ftsInfo) {
	r := w.r
	if r.opt.Debug&DebugSearch == 0 {
		return
	}
	prev := r.prevDepth.Swap(int64(e.depth))
	r.debugf("consider_visiting (early): %s: fts_info=%-6s, fts_level=%2d, prev_depth=%d fts_path=%s, fts_accpath=%s\n",
		r.quote(e.path), ftsInfoNames[info], e.depth, prev, r.quoteN(e.path), r.quoteN(e.accPath))
}

// symlinkLoop is ftsfind.c symlink_loop: options.xstat (so it shows under
// -D stat) fails with ELOOP.
func (w *walker) symlinkLoop(e *Entry) bool {
	_, err := w.r.xstat(e)
	return err == unix.ELOOP
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// considerRoot considers a start point and reports whether to walk below it.
func (w *walker) considerRoot(root *Entry, info ftsInfo, err error) bool {
	return w.consider(root, info, err) && info == ftsD
}

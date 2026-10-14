package find

import (
	"io/fs"
	"strings"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

const (
	unixS_IFMT  = unix.S_IFMT
	unixS_IFDIR = unix.S_IFDIR
	unixS_IFLNK = unix.S_IFLNK
	unixS_IFREG = unix.S_IFREG
)

// ftsInfo is the fts_info value consider_visiting switches on.
type ftsInfo uint8

const (
	ftsD ftsInfo = iota
	ftsDC
	ftsDNR
	ftsDP
	ftsERR
	ftsF
	ftsNS
	ftsNSOK
	ftsSL
	ftsSLNONE
	ftsDEFAULT
)

var ftsInfoNames = [...]string{"FTS_D", "FTS_DC", "FTS_DNR", "FTS_DP", "FTS_ERR",
	"FTS_F", "FTS_NS", "FTS_NSOK", "FTS_SL", "FTS_SLNONE", "FTS_DEFAULT"}

// statInfo is the fts_info fts_stat gives a successfully stat'ed entry.
// FTS_DEFAULT (FIFOs, sockets, devices) behaves like FTS_F everywhere but
// in -D search narration.
func statInfo(mode uint32) ftsInfo {
	switch mode & unix.S_IFMT {
	case unix.S_IFDIR:
		return ftsD
	case unix.S_IFLNK:
		return ftsSL
	case unix.S_IFREG:
		return ftsF
	}
	return ftsDEFAULT
}

// Entry is GNU's per-file `state` plus the fts path fields.
type Entry struct {
	path          string // fts_path, as printed and quoted in messages
	sysPath       string // the path every system call uses: fastwalk's, always shorter than PATH_MAX (Task 15)
	name          string // fts_name: the directory entry's name; the argument at depth 0
	accPath       string // fts_accpath = state.rel_pathname: name at depth >= 1 (full path under -L), the argument at 0
	depth         int    // fts_level
	startLen      int    // state.starting_path_length: the length of this entry's start point (%H, %P)
	haveStat      bool
	haveType      bool
	mode          uint32 // state.type (S_IFMT bits, full st_mode once stat'ed)
	st            unix.Stat_t
	issuedStatErr bool        // already_issued_stat_error_msg
	prune         bool        // stop_at_current_level
	cancelled     atomic.Bool // fts will never return this directory's FTS_DP (FTS_ERR, vanished under -ignore_readdir_race)
	txn           txn
	record        recordState // an open -printf/-fprintf record (record.go)
}

// setType records a d_type-derived type without a stat (FTS_NSOK).
func (e *Entry) setType(m fs.FileMode) {
	switch {
	case m.IsDir():
		e.mode = unix.S_IFDIR
	case m&fs.ModeSymlink != 0:
		e.mode = unix.S_IFLNK
	case m&fs.ModeNamedPipe != 0:
		e.mode = unix.S_IFIFO
	case m&fs.ModeSocket != 0:
		e.mode = unix.S_IFSOCK
	case m&fs.ModeCharDevice != 0:
		e.mode = unix.S_IFCHR
	case m&fs.ModeDevice != 0:
		e.mode = unix.S_IFBLK
	default:
		e.mode = unix.S_IFREG
	}
	e.haveType = true
}

func (e *Entry) setStat(st *unix.Stat_t) {
	e.st, e.haveStat, e.haveType, e.mode = *st, true, true, st.Mode
}

func (e *Entry) isDir() bool { return e.mode&unix.S_IFMT == unix.S_IFDIR }

// dirPath is the working directory -execdir uses for the entry: its parent
// directory below a start point (record_exec_dir's directory relative to
// fts's cwd), gnulib mdir_name(argument) for the start point itself. It is
// a system-call path: below a start point it comes from sysPath, which
// fastwalk builds as parent + "/" + name.
func (e *Entry) dirPath() string {
	if e.depth > 0 {
		d := strings.TrimRight(e.sysPath[:len(e.sysPath)-len(e.name)], "/")
		if d == "" {
			return "/"
		}
		return d
	}
	return mdirName(e.path)
}

// mdirName is gnulib mdir_name: the directory part without trailing
// slashes, "." when there is none, "/" for the root ("/" → "/",
// "tmp/top/" → "tmp", "a" → ".", "/a" → "/", "a//b" → "a").
func mdirName(p string) string {
	end := len(p)
	for end > 1 && p[end-1] == '/' {
		end--
	}
	q := p[:end]
	i := strings.LastIndexByte(q, '/')
	switch {
	case i < 0:
		return "."
	case i == 0:
		return "/"
	}
	for i > 1 && q[i-1] == '/' {
		i--
	}
	return q[:i]
}

// getInfo is util.c get_info.
func (r *Run) getInfo(e *Entry, p *Predicate) bool {
	todo := false
	switch {
	case p.needStat && !e.haveStat:
		todo = true
	case p.needType && !e.haveType:
		todo = true
	case p.needInum:
		todo = !e.haveStat // fastwalk exposes no d_ino, so stat for the inode
	}
	if todo {
		return r.getStatinfo(e)
	}
	return true
}

// getStatinfo is util.c get_statinfo.
func (r *Run) getStatinfo(e *Entry) bool {
	if !e.haveStat {
		st, err := r.xstat(e)
		if err != nil {
			if !r.opt.IgnoreReaddirRace || !isENOENT(err) {
				r.nonfatalTargetFileError(e, err)
			}
			return false
		}
		if st.Mode == 0 {
			r.errorAt(e, nil, "WARNING: file %s appears to have mode 0000", r.quote(e.accPath)) // GNU quotes `name`
			r.setFailure()
		}
		e.st = st
	}
	e.haveStat, e.haveType = true, true
	e.mode = e.st.Mode
	return true
}

// xstat is options.xstat for an entry: the syscall uses the full path
// (qfind has no fts cwd), while -D stat shows fts_accpath as GNU does.
func (r *Run) xstat(e *Entry) (unix.Stat_t, error) {
	return r.xstatName(e.sysPath, e.accPath, e.depth)
}

// xstatName is options.xstat — optionp_stat, optionh_stat or optionl_stat,
// wrapped by debug_stat under -D stat (util.c). depth is state.curdepth:
// 0 for command-line arguments such as -newer's reference file. Only these
// stats print debug_stat; the traversal's own stats (fts's) never do.
func (r *Run) xstatName(path, shown string, depth int) (unix.Stat_t, error) {
	var st unix.Stat_t
	if r.opt.Debug&DebugStat != 0 {
		r.debugf("debug_stat (%s)\n", shown)
	}
	follow := r.opt.Symlinks == SymlinkAlwaysDeref ||
		(r.opt.Symlinks == SymlinkDerefArgsOnly && depth == 0)
	if !follow {
		return st, unix.Lstat(path, &st)
	}
	err := unix.Stat(path, &st)
	if err == unix.ENOENT || err == unix.ENOTDIR { // fallback_stat
		if r.opt.Debug&DebugStat != 0 {
			r.debugf("fallback_stat(): stat(%s) failed; falling back on lstat()\n", shown)
		}
		return st, unix.Lstat(path, &st)
	}
	return st, err
}

// nonfatalTargetFileError is util.c report_file_err(0, errno, true, path):
// one message per entry, after e's earlier output (see errorAt).
func (r *Run) nonfatalTargetFileError(e *Entry, err error) {
	if !e.issuedStatErr {
		r.errorAt(e, err, "%s", r.quote(e.path))
		r.setFailure()
	}
	e.issuedStatErr = true
}

func isENOENT(err error) bool { return err == unix.ENOENT }

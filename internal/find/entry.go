package find

import "golang.org/x/sys/unix"

const (
	unixS_IFMT  = unix.S_IFMT
	unixS_IFDIR = unix.S_IFDIR
	unixS_IFLNK = unix.S_IFLNK
	unixS_IFREG = unix.S_IFREG
)

// Entry is GNU's per-file `state` plus the fts path fields.
type Entry struct {
	path          string // fts_path, as printed and quoted in messages
	sysPath       string // the path every system call uses: fastwalk's, always shorter than PATH_MAX (Task 15)
	name          string // fts_name: the directory entry's name; the argument at depth 0
	accPath       string // fts_accpath = state.rel_pathname: name at depth >= 1 (full path under -L), the argument at 0
	depth         int    // fts_level
	haveStat      bool
	haveType      bool
	mode          uint32 // state.type (S_IFMT bits, full st_mode once stat'ed)
	st            unix.Stat_t
	issuedStatErr bool // already_issued_stat_error_msg
	prune         bool // stop_at_current_level
	txn           txn
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
			r.errorf(nil, "WARNING: file %s appears to have mode 0000", r.quote(e.accPath)) // GNU quotes `name`
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
// one message per entry.
func (r *Run) nonfatalTargetFileError(e *Entry, err error) {
	if !e.issuedStatErr {
		r.errorf(err, "%s", r.quote(e.path))
		r.setFailure()
	}
	e.issuedStatErr = true
}

func isENOENT(err error) bool { return err == unix.ENOENT }

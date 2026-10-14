package find

import "golang.org/x/sys/unix"

// fileType is defs.h enum file_type, the index of predArgs.types.
type fileType uint8

const (
	ftypeBlk fileType = iota
	ftypeChr
	ftypeDir
	ftypeReg
	ftypeLnk
	ftypeFifo
	ftypeSock
	ftypeDoor
	ftypeCount
)

// insertType is parser.c insert_type, shared by -type and -xtype.
func (ps *parser) insertType(e *parserEntry, fn PredFunc, id FuncID) bool {
	r := ps.r
	name := "-type"
	if id == fnXtype {
		name = "-xtype"
	}
	arg, ok := ps.collectArg()
	if !ok {
		return false
	}
	if arg == "" {
		r.fatalf(nil, "Arguments to %s should contain at least one letter", name)
	}
	p := ps.insertPrimaryWithPred(e, fn, id, arg, true)
	p.estSuccessRate = 0
	if id == fnXtype {
		p.needStat, p.needType = true, false
	} else {
		p.needStat, p.needType = false, true // struct dirent is enough
	}
	for i := 0; i < len(arg); {
		var cell fileType
		var rate float32
		switch arg[i] {
		case 'b':
			cell, rate = ftypeBlk, 0.000888
		case 'c':
			cell, rate = ftypeChr, 0.000443
		case 'd':
			cell, rate = ftypeDir, 0.0922
		case 'f':
			cell, rate = ftypeReg, 0.875
		case 'l':
			cell, rate = ftypeLnk, 0.0311
		case 'p':
			cell, rate = ftypeFifo, 7.554e-6
		case 's':
			cell, rate = ftypeSock, 1.59e-5
		case 'D': // Linux has no S_IFDOOR
			r.fatalf(nil, "%s %s is not supported because Solaris doors are not supported on the platform find was compiled on.", name, cbyte(arg[i]))
		default:
			r.fatalf(nil, "Unknown argument to %s: %s", name, cbyte(arg[i]))
		}
		if p.args.types[cell] {
			r.fatalf(nil, "Duplicate file type '%s' in the argument list to %s.", cbyte(arg[i]), name)
		}
		p.estSuccessRate += rate
		p.args.types[cell] = true
		i++
		if i < len(arg) {
			if arg[i] != ',' {
				r.fatalf(nil, "Must separate multiple arguments to %s using: ','", name)
			}
			i++
			if i == len(arg) {
				r.fatalf(nil, "Last file type in list argument to %s is missing, i.e., list is ending on: ','", name)
			}
		}
	}
	return true
}

func parseType(ps *parser, e *parserEntry) bool  { return ps.insertType(e, predType, fnType) }
func parseXtype(ps *parser, e *parserEntry) bool { return ps.insertType(e, predXtype, fnXtype) }

// modeToFiletype is the S_IFMT switch of pred.c pred_type; false for a
// mode of no known type (FTYPE_COUNT).
func modeToFiletype(mode uint32) (fileType, bool) {
	switch mode & unix.S_IFMT {
	case unix.S_IFREG:
		return ftypeReg, true
	case unix.S_IFDIR:
		return ftypeDir, true
	case unix.S_IFLNK:
		return ftypeLnk, true
	case unix.S_IFBLK:
		return ftypeBlk, true
	case unix.S_IFCHR:
		return ftypeChr, true
	case unix.S_IFSOCK:
		return ftypeSock, true
	case unix.S_IFIFO:
		return ftypeFifo, true
	}
	return ftypeCount, false
}

// typeLetter is print.c mode_to_filetype for %y/%Y.
func typeLetter(mode uint32) byte {
	switch mode & unix.S_IFMT {
	case unix.S_IFREG:
		return 'f'
	case unix.S_IFDIR:
		return 'd'
	case unix.S_IFLNK:
		return 'l'
	case unix.S_IFSOCK:
		return 's'
	case unix.S_IFBLK:
		return 'b'
	case unix.S_IFCHR:
		return 'c'
	case unix.S_IFIFO:
		return 'p'
	}
	return 'U'
}

// predType is pred.c pred_type: the stat mode when known, else d_type.
func predType(r *Run, e *Entry, p *Predicate) bool {
	return typeMatches(e, e.st.Mode, p)
}

// typeMatches is the body of pred_type with stat_buf's mode passed in:
// pred_xtype hands it its own stat, used only once the entry has a stat.
func typeMatches(e *Entry, statMode uint32, p *Predicate) bool {
	if e.mode == 0 { // broken NFS servers (Savannah #16378)
		return false
	}
	mode := e.mode
	if e.haveStat {
		mode = statMode
	}
	t, ok := modeToFiletype(mode)
	return ok && p.args.types[t]
}

// followingLinks is util.c following_links.
func (r *Run) followingLinks(e *Entry) bool {
	switch r.opt.Symlinks {
	case SymlinkAlwaysDeref:
		return true
	case SymlinkDerefArgsOnly:
		return e.depth == 0
	}
	return false
}

// predXtype is pred.c pred_xtype: stat the opposite way to the current
// symlink policy, calling optionl_stat or optionp_stat directly (never
// debug_stat), and fall back to the entry's own type when a link is broken
// (err_signals_broken_link: ENOENT or ELOOP), as ls -lL does.
func predXtype(r *Run, e *Entry, p *Predicate) bool {
	var st unix.Stat_t
	follows := !r.followingLinks(e)
	var err error
	if follows {
		err = unix.Stat(e.sysPath, &st)
		if err == unix.ENOENT || err == unix.ENOTDIR { // fallback_stat
			if r.opt.Debug&DebugStat != 0 {
				r.debugf("fallback_stat(): stat(%s) failed; falling back on lstat()\n", e.accPath)
			}
			err = unix.Lstat(e.sysPath, &st)
		}
	} else {
		err = unix.Lstat(e.sysPath, &st)
	}
	if err != nil {
		if follows && (err == unix.ELOOP || err == unix.ENOENT) {
			return predType(r, e, p)
		}
		r.errorAt(e, err, "%s", r.quote(e.path))
		r.setFailure()
		return false
	}
	return typeMatches(e, st.Mode, p)
}

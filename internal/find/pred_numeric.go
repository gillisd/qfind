package find

import "golang.org/x/sys/unix"

func predInum(r *Run, e *Entry, p *Predicate) bool {
	return compareNum(p.args.num.kind, p.args.num.val, e.st.Ino)
}

func predLinks(r *Run, e *Entry, p *Predicate) bool {
	return compareNum(p.args.num.kind, p.args.num.val, uint64(e.st.Nlink))
}

func predUid(r *Run, e *Entry, p *Predicate) bool {
	return compareNum(p.args.num.kind, p.args.num.val, uint64(e.st.Uid))
}

func predGid(r *Run, e *Entry, p *Predicate) bool {
	return compareNum(p.args.num.kind, p.args.num.val, uint64(e.st.Gid))
}

// predSize is pred.c pred_size: the size in units, rounded up.
func predSize(r *Run, e *Entry, p *Predicate) bool {
	bs := p.args.size.blockSize
	f := e.st.Size / bs
	if e.st.Size%bs != 0 {
		f++
	}
	return compareNum(p.args.size.kind, p.args.size.size, uint64(f))
}

// predSamefile is pred.c pred_samefile (need_stat: the stat is current).
func predSamefile(r *Run, e *Entry, p *Predicate) bool {
	return e.st.Ino == p.args.samefile.ino && e.st.Dev == p.args.samefile.dev
}

// predEmpty is pred.c pred_empty: a directory with no entries but . and ..
// (read now, so under -depth it sees the state after its contents were
// processed), or a regular file of size 0.
func predEmpty(r *Run, e *Entry, p *Predicate) bool {
	switch e.st.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
		empty, err := dirIsEmpty(e.sysPath)
		if err != nil {
			r.errorAt(e, err, "%s", r.quote(e.path))
			r.setFailure()
			return false
		}
		return empty
	case unix.S_IFREG:
		return e.st.Size == 0
	}
	return false
}

// dirIsEmpty is pred_empty's openat/fdopendir/readdir/closedir sequence.
func dirIsEmpty(path string) (bool, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		return false, err
	}
	buf := make([]byte, 8192)
	empty := true
	for empty {
		n, err := unix.Getdents(fd, buf)
		if err != nil {
			unix.Close(fd)
			return false, err
		}
		if n == 0 {
			break
		}
		var names []string
		_, _, names = unix.ParseDirent(buf[:n], -1, names) // skips "." and ".."
		empty = len(names) == 0
	}
	if err := unix.Close(fd); err != nil {
		return false, err
	}
	return empty, nil
}

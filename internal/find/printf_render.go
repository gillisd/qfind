package find

import (
	"math"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"qfind/internal/filemode"
	"qfind/internal/idcache"
	"qfind/internal/mounts"
)

// predFprintf is print.c pred_fprintf (-printf and -fprintf). Every
// segment is one checked stdio call (checked_fwrite, checked_fprintf,
// checked_fflush): a failed write is diagnosed under the stream's name.
func predFprintf(r *Run, e *Entry, p *Predicate) bool {
	fv := p.args.format
	r.openRecord(e, fv.out)
	for i := range fv.segments {
		seg := &fv.segments[i]
		switch seg.kind {
		case segPlain:
			r.emitChecked(e, fv.out, seg.text, failReported)
		case segStop: // \c: the last segment; flush the format's stream
			r.emitChecked(e, fv.out, seg.text, failReported)
			r.flushStream(e, fv.out, failReported)
		default:
			text, after := r.renderDirective(e, fv.out, seg)
			switch {
			case seg.conv == '%': // checked_fwrite, not a printf
				r.emitChecked(e, fv.out, text, failReported)
			case seg.spec.overflow:
				// vfprintf fails with EOVERFLOW before writing anything.
				r.errorAt(e, unix.EOVERFLOW, "%s", r.quote(fv.out.name))
				r.setFailure()
			default:
				r.emitChecked(e, fv.out, text, failReported)
			}
			if after != nil {
				after()
			}
		}
	}
	r.closeRecord(e)
	return true
}

func tsOf(t unix.Timespec) Timespec { return Timespec{t.Sec, t.Nsec} }

// renderDirective is do_fprintf's KIND_FORMAT case, with pred_fprintf's
// date components: the text of the one fprintf call, which starts with
// the segment's literal prefix. Diagnostics GNU issues before that call
// are issued here; after, when not nil, issues those GNU issues once the
// call has been made (glibc error() flushes the text out first).
func (r *Run) renderDirective(e *Entry, out *Stream, seg *segment) (text string, after func()) {
	st := &e.st
	str := func(v string) string { return seg.text + cfmtS(seg.spec, v) }
	quoted := func(v string) string { // checked_print_quoted
		if out.tty {
			v = r.loc.Qmark(v)
		}
		return str(v)
	}
	num := func(v uint64) string { return str(strconv.FormatUint(v, 10)) } // human_readable(v, …, 1, 1)
	if seg.fc2 != 0 {                                                      // %A? %B? %C? %T?
		var ts Timespec
		switch seg.fc {
		case 'A':
			ts = tsOf(st.Atim)
		case 'C':
			ts = tsOf(st.Ctim)
		case 'T':
			ts = tsOf(st.Mtim)
		case 'B': // get_stat_birthtime without st_birthtim: {-1, -1}, valid for '@' only
			if seg.fc2 != '@' {
				return str(""), nil
			}
			ts = Timespec{-1, -1}
		}
		return str(r.formatDate(ts, seg.fc2)), nil
	}
	switch seg.fc {
	case 'a':
		return str(r.ctimeFormat(tsOf(st.Atim))), nil
	case 'c':
		return str(r.ctimeFormat(tsOf(st.Ctim))), nil
	case 't':
		return str(r.ctimeFormat(tsOf(st.Mtim))), nil
	case 'b': // human_readable(st_blocks, ceiling, 512, 512)
		return num(uint64(st.Blocks)), nil
	case 'k': // human_readable(st_blocks, ceiling, 512, 1024)
		return num((uint64(st.Blocks) + 1) / 2), nil
	case 'd':
		return seg.text + cfmtD(seg.spec, int64(e.depth)), nil
	case 'D':
		return num(st.Dev), nil
	case 'f':
		return quoted(baseName(e.path)), nil
	case 'F':
		return quoted(r.fsType(e)), nil
	case 'g':
		if name, ok := idcache.GroupName(st.Gid); ok {
			return str(name), nil
		}
		return num(uint64(st.Gid)), nil
	case 'G':
		return num(uint64(st.Gid)), nil
	case 'h':
		return quoted(dirnameH(e.path)), nil
	case 'H':
		return str(e.path[:e.startLen]), nil
	case 'i':
		return num(st.Ino), nil
	case 'l':
		target := ""
		if st.Mode&unix.S_IFMT == unix.S_IFLNK {
			t, err := readlink(e.sysPath)
			if err != nil {
				r.nonfatalTargetFileError(e, err)
				r.setFailure()
			}
			target = t
		}
		return quoted(target), nil
	case 'M':
		return str(filemode.String(st.Mode)), nil
	case 'm':
		return seg.text + cfmtO(seg.spec, uint64(st.Mode&0o7777)), nil
	case 'n':
		return num(uint64(st.Nlink)), nil
	case 'p':
		return quoted(e.path), nil
	case 'P':
		rest := ""
		if e.depth > 0 {
			rest = e.path[e.startLen:]
			if rest != "" && rest[0] == '/' { // the separator fts added
				rest = rest[1:]
			}
		}
		return quoted(rest), nil
	case 's':
		return num(uint64(st.Size)), nil
	case 'S':
		return seg.text + cfmtG(seg.spec, sparseness(st)), nil
	case 'u':
		if name, ok := idcache.UserName(st.Uid); ok {
			return str(name), nil
		}
		return num(uint64(st.Uid)), nil
	case 'U':
		return num(uint64(st.Uid)), nil
	case 'Y':
		return r.linkTargetType(e, str)
	case 'y':
		return str(string(typeLetter(e.mode))), nil
	case 'Z':
		ctx, err := r.getfilecon(e)
		if err != nil {
			// %Z expands to nothing; the message follows the text.
			return str(""), func() {
				r.errorAt(e, err, "getfilecon failed: %s", r.quote(e.path))
				r.setFailure()
			}
		}
		return str(ctx), nil
	case '%':
		return seg.raw, nil
	}
	return seg.text, nil
}

// sparseness is print.c file_sparseness.
func sparseness(st *unix.Stat_t) float64 {
	if st.Size == 0 {
		switch {
		case st.Blocks == 0:
			return 1.0
		case st.Blocks < 0:
			return math.Inf(-1)
		}
		return math.Inf(1)
	}
	return 512 * float64(st.Blocks) / float64(st.Size)
}

// linkTargetType is %Y: the target's type letter for a symlink, N when it
// dangles (ENOENT/ENOTDIR), L for a loop; any other error prints '?' and
// then a diagnostic, without changing the exit status (print.c).
func (r *Run) linkTargetType(e *Entry, str func(string) string) (string, func()) {
	if e.st.Mode&unix.S_IFMT != unix.S_IFLNK {
		return str(string(typeLetter(e.st.Mode))), nil
	}
	var st unix.Stat_t
	switch err := unix.Stat(e.sysPath, &st); err {
	case nil:
		return str(string(typeLetter(st.Mode))), nil
	case unix.ENOENT, unix.ENOTDIR:
		return str("N"), nil
	case unix.ELOOP:
		return str("L"), nil
	default:
		return str("?"), func() { r.errorAt(e, err, "%s", r.quote(e.path)) }
	}
}

// fsType is fstype.c filesystem_type for %F and -fstype.
func (r *Run) fsType(e *Entry) string {
	t, ok := mounts.FSType(e.st.Dev)
	if !ok {
		r.fatalAt(e, nil, "Cannot read mounted file system list")
	}
	return t
}

// getfilecon is options.x_getfilecon: libselinux lgetfilecon under -P
// (and below the start points under -H), getfilecon with
// fallback_getfilecon's lgetfilecon on ENOENT/ENOTDIR otherwise. It is the
// security.selinux attribute without its trailing NUL.
func (r *Run) getfilecon(e *Entry) (string, error) {
	if !r.followingLinks(e) {
		return getxattrCon(unix.Lgetxattr, e.sysPath)
	}
	ctx, err := getxattrCon(unix.Getxattr, e.sysPath)
	if err == unix.ENOENT || err == unix.ENOTDIR {
		if r.opt.Debug&DebugStat != 0 {
			r.debugf("fallback_getfilecon(): getfilecon(%s) failed; falling back on lgetfilecon()\n", e.accPath)
		}
		return getxattrCon(unix.Lgetxattr, e.sysPath)
	}
	return ctx, err
}

// getxattrCon is libselinux getfilecon_raw: grow the buffer on ERANGE; an
// empty attribute is ENOTSUP.
func getxattrCon(get func(string, string, []byte) (int, error), path string) (string, error) {
	buf := make([]byte, 255)
	for {
		n, err := get(path, "security.selinux", buf)
		if err == unix.ERANGE {
			buf = make([]byte, 2*len(buf))
			continue
		}
		if err != nil {
			return "", err
		}
		if n == 0 {
			return "", unix.ENOTSUP
		}
		return strings.TrimSuffix(string(buf[:n]), "\x00"), nil
	}
}

// dirnameH is print.c's %h: trailing slashes go unless only one character
// precedes them, then everything from the last slash.
func dirnameH(path string) string {
	s := len(path) - 1
	for s >= 0 && path[s] == '/' {
		s--
	}
	p := path
	if s > 0 && s+1 < len(path) && path[s+1] == '/' {
		p = path[:s+1]
	}
	i := strings.LastIndexByte(p, '/')
	if i < 0 {
		return "."
	}
	return p[:i]
}

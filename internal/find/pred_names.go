package find

import (
	"strings"

	"golang.org/x/sys/unix"

	"qfind/internal/fnmatch"
)

// fnOpts is the fnmatch flags (FNM_CASEFOLD or none) plus the locale and
// POSIXLY_CORRECT state glibc's fnmatch consults.
func (r *Run) fnOpts(fold bool) fnmatch.Options {
	return fnmatch.Options{CaseFold: fold, UTF8: r.loc.UTF8, PosixlyCorrect: r.opt.PosixlyCorrect}
}

// fnmatchRun is fnmatch(pattern, s, fold ? FNM_CASEFOLD : 0) == 0.
func fnmatchRun(r *Run, pattern, s string, fold bool) bool {
	return fnmatch.Match(pattern, s, r.fnOpts(fold))
}

// baseName is gnulib base_name: the last component, keeping one trailing
// slash ("dir/" → "dir/", "a//" → "a/"), and "/" for an all-slash path.
func baseName(path string) string {
	i := len(path)
	for i > 0 && path[i-1] == '/' {
		i--
	}
	if i == 0 {
		if path == "" {
			return ""
		}
		return "/"
	}
	start := strings.LastIndexByte(path[:i], '/') + 1
	if i < len(path) {
		return path[start : i+1]
	}
	return path[start:i]
}

// stripTrailingSlashes is gnulib strip_trailing_slashes ("/" stays "/").
func stripTrailingSlashes(s string) string {
	t := strings.TrimRight(s, "/")
	if t == "" && s != "" {
		return "/"
	}
	return t
}

// predNameCommon is pred.c pred_name_common: match the base name without
// trailing slashes; FNM_PERIOD is not used, as POSIX requires.
func (r *Run) predNameCommon(path, pattern string, fold bool) bool {
	return fnmatchRun(r, pattern, stripTrailingSlashes(baseName(path)), fold)
}

func predName(r *Run, e *Entry, p *Predicate) bool {
	return r.predNameCommon(e.path, p.args.str, false)
}
func predIname(r *Run, e *Entry, p *Predicate) bool {
	return r.predNameCommon(e.path, p.args.str, true)
}

// predPath is pred.c pred_path (-path, -wholename).
func predPath(r *Run, e *Entry, p *Predicate) bool { return fnmatchRun(r, p.args.str, e.path, false) }

// predIpath is pred.c pred_ipath (-ipath, -iwholename).
func predIpath(r *Run, e *Entry, p *Predicate) bool { return fnmatchRun(r, p.args.str, e.path, true) }

// matchLname is pred.c match_lname: only an entry whose stat says symlink
// (so under -L only a dangling one) has a target to match.
func (r *Run) matchLname(e *Entry, p *Predicate, fold bool) bool {
	if e.st.Mode&unix.S_IFMT != unix.S_IFLNK {
		return false
	}
	target, err := readlink(e.sysPath)
	if err != nil {
		r.nonfatalTargetFileError(e, err)
		r.setFailure()
		return false
	}
	return fnmatchRun(r, p.args.str, target, fold)
}

func predLname(r *Run, e *Entry, p *Predicate) bool  { return r.matchLname(e, p, false) }
func predIlname(r *Run, e *Entry, p *Predicate) bool { return r.matchLname(e, p, true) }

// readlink is gnulib areadlinkat: readlink with a buffer that grows until
// the target fits.
func readlink(path string) (string, error) {
	for size := 1024; ; size *= 2 {
		buf := make([]byte, size)
		n, err := unix.Readlink(path, buf)
		if err != nil {
			return "", err
		}
		if n < size {
			return string(buf[:n]), nil
		}
	}
}

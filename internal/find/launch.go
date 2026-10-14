package find

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"qfind/internal/libc"
)

// launch is exec.c launch: flush, spawn, wait. It returns the child's exit
// status (1 when the child could not run the command, 128+sig for a
// signal). Callers hold evalMu (serial mode) or run after the walk, so one
// child runs at a time; execMu makes that independent of the evaluation
// mode.
func (r *Run) launch(e *Entry, ev *execVal, argv []string, dir string) int {
	r.execMu.Lock()
	defer r.execMu.Unlock()
	if r.opt.Debug&DebugExec != 0 {
		var b strings.Builder
		for _, a := range argv {
			b.WriteString(" " + r.quote(a))
		}
		r.debugf("DebugExec: launching process (argc=%d):%s\n", len(argv), b.String())
	}
	r.flushStdout(e) // fflush(stdout) before fork; stderr is unbuffered

	// The child's own work before execvp (prep_child_for_exec, the leak
	// check, the testing limits) is done here; its messages and _exit(1)
	// become the parent's message and status 1.
	if dir != "" {
		// restore_cwd: fchdir needs search permission.
		if err := unix.Access(dir, unix.X_OK); err != nil {
			r.errorf(err, "Failed to change directory")
			return 1
		}
	}
	if ev.closeStdin && r.leakCheck() {
		return r.emulateLeakAbort(e, ev, argv)
	}
	if msg, ok := r.argsExceedTestingLimit(argv); ok {
		r.writeStderr(msg)
		return 1
	}

	path, err := r.execvpPath(argv[0], dir)
	var pid int
	if err == nil {
		files, closeNull := r.childFiles(e, ev)
		attr := &syscall.ProcAttr{Dir: dir, Env: r.env, Files: files}
		pid, _, err = syscall.StartProcess(path, argv, attr)
		if err == unix.ENOEXEC { // execvp's fallback for a script without #!
			pid, _, err = syscall.StartProcess("/bin/sh", append([]string{"/bin/sh", path}, argv[1:]...), attr)
		}
		closeNull()
	}
	if err != nil {
		errno, _ := libc.Errno(err)
		if errno == unix.EAGAIN || errno == unix.ENOMEM {
			r.fatalAt(e, errno, "cannot fork")
		}
		r.errorf(errno, "%s", r.quote(argv[0])) // the child's message in GNU
		return 1
	}
	var ws syscall.WaitStatus
	for {
		_, err = syscall.Wait4(pid, &ws, 0, nil)
		if err != unix.EINTR {
			break
		}
	}
	if err != nil {
		r.errorf(err, "error waiting for the command we executed for %s", r.quote(argv[0]))
		r.setFailure()
		return 1
	}
	if ws.Signaled() {
		r.errorf(nil, "%s terminated by signal %d", r.quote(argv[0]), int(ws.Signal()))
		return 128 + int(ws.Signal())
	}
	if r.opt.Debug&DebugExec != 0 {
		r.debugf("DebugExec: process (PID=%d) terminated with exit status: %d\n", pid, ws.ExitStatus())
	}
	return ws.ExitStatus()
}

// execvpPath resolves argv[0] like glibc execvp: names with '/' are used
// as given (relative to dir); otherwise PATH (default "/bin:/usr/bin" when
// unset; "" means ".") is searched, skipping EACCES candidates.
func (r *Run) execvpPath(name, dir string) (string, error) {
	if strings.Contains(name, "/") {
		return name, nil
	}
	if name == "" {
		return "", unix.ENOENT
	}
	path, ok := r.lookupEnv("PATH")
	if !ok {
		path = "/bin:/usr/bin"
	}
	sawEACCES := false
	for _, d := range strings.Split(path, ":") {
		if d == "" {
			d = "."
		}
		cand := d + "/" + name
		probe := cand
		if dir != "" && !strings.HasPrefix(cand, "/") {
			probe = dir + "/" + cand
		}
		var st unix.Stat_t
		if err := unix.Stat(probe, &st); err != nil {
			if err == unix.EACCES {
				sawEACCES = true
			}
			continue
		}
		if st.Mode&unix.S_IFMT != unix.S_IFREG || unix.Access(probe, unix.X_OK) != nil {
			sawEACCES = true
			continue
		}
		return cand, nil
	}
	if sawEACCES {
		return "", unix.EACCES
	}
	return "", unix.ENOENT
}

func (r *Run) leakCheck() bool { _, ok := r.lookupEnv("GNU_FINDUTILS_FD_LEAK_CHECK"); return ok }

// emulateLeakAbort reproduces the -ok/-okdir child under GNU's descriptor
// leak check: lib/fdleak.c complains about the /dev/null that open_safer
// put above fd 2, the assertion fails, and the child dies of SIGABRT.
// This keeps the documented-bug examples pending. glibc's assert message
// names __progname (the base name).
func (r *Run) emulateLeakAbort(e *Entry, ev *execVal, argv []string) int {
	r.errorf(nil, "File descriptor %d will leak; please report this as a bug, remembering to include a detailed description of the simplest way to reproduce this problem.", r.okNullFD(e, ev))
	r.debugf("%s: fdleak.c:395: complain_about_leaky_fds: Assertion `no_leaks' failed.\n", filepath.Base(r.prog))
	r.errorf(nil, "%s terminated by signal %d", r.quote(argv[0]), int(unix.SIGABRT))
	return 128 + int(unix.SIGABRT)
}

// okNullFD is the descriptor the 4.11.0 -ok/-okdir child reopens
// /dev/null on: open_safer's lowest free one above 2 while GNU's own
// close-on-exec descriptors are still open in the child. Those are the
// initial working directory (3), fts's two below a start point, and for
// -okdir the recorded directory plus fts's cached ancestors (measured on
// the oracle). Descriptors find inherited sit below them.
func (r *Run) okNullFD(e *Entry, ev *execVal) int {
	n := 4
	for fd := range r.inheritedFDs {
		if fd > 2 {
			n++
		}
	}
	if e != nil && e.depth > 0 {
		n += 2
	}
	if ev.local {
		n++
		if e != nil && e.depth > 1 {
			n += e.depth - 1
		}
	}
	return n
}

// recordInheritedFDs remembers the descriptors find started with that a
// child would inherit (fdleak.c remember_non_cloexec_fds). Main calls it
// first.
func (r *Run) recordInheritedFDs() {
	r.inheritedFDs = map[int]bool{}
	fds, _ := openFDs()
	for _, fd := range fds {
		if flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); err == nil && flags&unix.FD_CLOEXEC == 0 {
			r.inheritedFDs[fd] = true
		}
	}
}

// fdSlack covers descriptors opened between childFiles' listing and the
// fork by goroutines other than fastwalk's workers (the runtime's own).
const fdSlack = 16

// childFiles builds the child's descriptor table. GNU's fts opens every
// directory close-on-exec, so a child inherits only what find itself
// inherited. fastwalk opens directories with plain open(2), and its
// workers may hold or open one at any moment, so qfind closes every other
// descriptor in the child instead (a -1 entry makes the forked child
// close that number before execve), which leaves no window for a race.
//
// A descriptor opened after the listing takes the lowest free number,
// which is at most the number of descriptors then open: those listed plus
// one per fastwalk worker (each holds at most one directory) plus slack.
// So every number up to that bound is either kept or closed.
//
// For -ok/-okdir it also reproduces the 4.11.0 bug: fd 0 closed and
// /dev/null on the descriptor open_safer chose. The returned func closes
// the parent's copy of /dev/null.
func (r *Run) childFiles(e *Entry, ev *execVal) ([]uintptr, func()) {
	const closed = ^uintptr(0)
	fds, err := openFDs()
	n := 3
	if err != nil {
		n = 1 << 16 // no /proc: close everything RLIMIT_NOFILE may allow
		var rl unix.Rlimit
		if unix.Getrlimit(unix.RLIMIT_NOFILE, &rl) == nil && rl.Cur < uint64(n) {
			n = int(rl.Cur)
		}
	}
	open := make(map[int]bool, len(fds))
	for _, fd := range fds {
		open[fd] = true
		n = max(n, fd+1)
	}
	n += r.workers + fdSlack
	nullFD, target := -1, -1
	if ev.closeStdin {
		target = r.okNullFD(e, ev)
		for r.inheritedFDs[target] {
			target++
		}
		n = max(n, target+1)
		fd, err := unix.Open(os.DevNull, unix.O_RDONLY|unix.O_CLOEXEC, 0)
		if err != nil {
			// prep_child_for_exec: not fatal, the child runs with fd 0 closed.
			r.errorf(err, "%s", r.quote(os.DevNull))
		}
		nullFD = fd
	}
	files := make([]uintptr, n)
	for i := range files {
		files[i] = closed
		if r.inheritedFDs[i] && (open[i] || err != nil) {
			files[i] = uintptr(i)
		}
	}
	if ev.closeStdin {
		files[0] = closed
		if nullFD >= 0 {
			files[target] = uintptr(nullFD)
		}
	}
	return files, func() {
		if nullFD >= 0 {
			unix.Close(nullFD)
		}
	}
}

// openFDs lists this process's open descriptors.
func openFDs() ([]int, error) {
	ents, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return nil, err
	}
	fds := make([]int, 0, len(ents))
	for _, d := range ents {
		if fd, err := strconv.Atoi(d.Name()); err == nil {
			fds = append(fds, fd)
		}
	}
	return fds, nil
}

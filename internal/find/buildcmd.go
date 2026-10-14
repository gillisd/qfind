package find

import (
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"qfind/internal/libc"
)

// bcControl is lib/buildcmd.h struct buildcmd_control, reduced to what
// find uses (no xargs options).
type bcControl struct {
	argMax      int64 // bc_use_sensible_arg_max: min(128 KiB, posix max), at least 4096
	maxArgCount int   // posix_arg_size_max / sizeof(char*) - 2
	initialArgc int
}

// bcState is struct buildcmd_state: one pending command line.
type bcState struct {
	argv         []string
	chars        int64 // cmd_argv_chars (each argument counts strlen+1)
	initialChars int64 // cmd_initial_argv_chars
	todo         bool
	dir          string // -execdir: working directory of the batch
}

// posixArgMin is _POSIX_ARG_MAX, the floor bc_use_sensible_arg_max applies.
const posixArgMin = 4096

// bcInit is bc_init_controlinfo(ctl, 2048) + bc_use_sensible_arg_max. It
// reports false when the environment leaves no room; the caller prints
// "The environment is too large for exec().".
func bcInit(env []string) (bcControl, bool) {
	var envSize int64
	for _, e := range env {
		envSize += int64(len(e)) + 1
	}
	posixMax := libc.ArgMax()
	if envSize > posixMax || 2048+envSize >= posixMax {
		return bcControl{}, false
	}
	posixMax -= envSize + 2048
	c := bcControl{maxArgCount: int(posixMax/8) - 2, argMax: 128 * 1024}
	if c.argMax > posixMax {
		c.argMax = posixMax
	} else if c.argMax < posixArgMin {
		c.argMax = posixArgMin
	}
	return c, true
}

// reset is bc_clear_args.
func (s *bcState) reset(c *bcControl) {
	s.argv = s.argv[:c.initialArgc]
	s.chars, s.todo = s.initialChars, false
}

// push is bc_push_arg for the entry e (nil for the initial arguments,
// pushed at parse time). exec runs and clears the pending command line
// (bc_do_exec); replacing marks the ';' form, whose ctl.replace_pat makes
// an overlong line fatal instead of a reason to run the command early.
func (c *bcControl) push(r *Run, e *Entry, s *bcState, arg, prefix string, initial, replacing bool, exec func()) {
	cost := int64(len(arg)+1) + int64(len(prefix))
	if s.chars+cost > c.argMax {
		if initial || len(s.argv) == c.initialArgc {
			r.fatalAt(e, nil, "cannot fit single argument within argument list size limit")
		}
		if replacing {
			r.fatalAt(e, nil, "argument list too long")
		}
		exec()
	}
	if len(s.argv) == c.maxArgCount {
		exec()
	}
	if !initial {
		s.todo = true
	}
	s.argv = append(s.argv, prefix+arg)
	s.chars += cost
	if len(s.argv) == c.maxArgCount && exec != nil {
		exec()
	}
	if initial {
		s.initialChars = s.chars
	}
}

// insert is bc_do_insert for one ';' template argument: every "{}" is
// replaced by prefix+target, within arg_max-1 bytes.
func (c *bcControl) insert(r *Run, e *Entry, tmpl, prefix, target string) string {
	left := c.argMax - 1
	var b []byte
	for {
		i := strings.Index(tmpl, "{}")
		n := len(tmpl)
		if i >= 0 {
			n = i
		}
		if left <= int64(n) {
			break
		}
		left -= int64(n)
		b = append(b, tmpl[:n]...)
		tmpl = tmpl[n:]
		if i >= 0 {
			if left <= int64(len(target)+len(prefix)) {
				break
			}
			left -= int64(len(target) + len(prefix))
			b = append(b, prefix...)
			b = append(b, target...)
			tmpl = tmpl[2:]
		}
		if tmpl == "" {
			return string(b)
		}
	}
	r.fatalAt(e, nil, "command too long")
	return ""
}

// argsExceedTestingLimit is bc_args_exceed_testing_limit: the test-suite
// hooks that make the child fail with E2BIG. It returns the message the
// child would print and exit 1 with, or "".
func (r *Run) argsExceedTestingLimit(argv []string) (string, bool) {
	var chars uint64
	for _, a := range argv {
		chars += uint64(len(a))
	}
	for _, lim := range []struct {
		name string
		n    uint64
	}{
		{"__GNU_FINDUTILS_EXEC_ARG_COUNT_LIMIT", uint64(len(argv))},
		{"__GNU_FINDUTILS_EXEC_ARG_LENGTH_LIMIT", chars},
	} {
		v, ok := r.lookupEnv(lim.name)
		if !ok {
			continue
		}
		limit, err := strconv.ParseUint(strings.TrimLeft(v, " \t\n\v\f\r"), 10, 64)
		if err != nil || strings.HasPrefix(strings.TrimLeft(v, " \t\n\v\f\r"), "-") {
			// xstrtoul failure: error (EXIT_FAILURE, errno, …) in the child.
			return r.prog + ": Environment variable " + lim.name + " is not set to a valid decimal number" +
				errnoSuffix(err) + "\n", true
		}
		if lim.n > limit {
			return r.prog + ": " + r.quote(argv[0]) + ": " + libc.Strerror(unix.E2BIG) + "\n", true
		}
	}
	return "", false
}

// errnoSuffix is the ": strerror" part of a failed xstrtoul: ERANGE for
// an overflow, otherwise errno is whatever it was (normally 0, "Success").
func errnoSuffix(err error) string {
	if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
		return ": " + libc.Strerror(unix.ERANGE)
	}
	return ""
}

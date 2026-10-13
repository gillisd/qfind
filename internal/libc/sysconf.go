package libc

import "golang.org/x/sys/unix"

const (
	// legacyArgMax is glibc's ARG_MAX floor (131072).
	legacyArgMax = 131072
	// stackArgMax is the kernel's cap on argv plus environment,
	// _STK_LIM / 4 * 3 (fs/exec.c), which glibc's sysconf applies too.
	stackArgMax = 8 << 20 / 4 * 3
)

// ArgMax is glibc sysconf(_SC_ARG_MAX): RLIMIT_STACK.cur / 4, capped at
// 6 MiB as the kernel caps it and never below 131072. An unlimited stack
// therefore gives 6291456, as `getconf ARG_MAX` reports.
func ArgMax() int64 {
	var rl unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_STACK, &rl); err != nil {
		return legacyArgMax
	}
	return max(legacyArgMax, int64(min(rl.Cur/4, stackArgMax)))
}

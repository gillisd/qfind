package find

import (
	"strconv"

	"github.com/charlievieth/fastwalk"
	"golang.org/x/sys/unix"
)

// numWorkers caps fastwalk's workers so that each worker's open directory
// fd, plus stdio, the runtime and one child at a time, fits RLIMIT_NOFILE
// (execdir_spec runs with rlimit_nofile 30). QFIND_WORKERS overrides it
// for benchmarking.
func (r *Run) numWorkers() int {
	if v, ok := r.lookupEnv("QFIND_WORKERS"); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	n := fastwalk.DefaultNumWorkers()
	var rl unix.Rlimit
	if unix.Getrlimit(unix.RLIMIT_NOFILE, &rl) == nil && rl.Cur != unix.RLIM_INFINITY {
		n = min(n, max(1, (int(rl.Cur)-16)/2))
	}
	return n
}

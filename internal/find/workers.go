package find

import (
	"runtime"
	"strconv"

	"github.com/charlievieth/fastwalk"
	"golang.org/x/sys/unix"
)

// numWorkers is four fastwalk workers per CPU, up to fastwalk's own cap
// of 32: a worker spends much of its time blocked in getdents, open or
// the walk's hand-offs, and on a 4-CPU machine 16 workers walk a warm
// tree 25% faster than fastwalk's default of one per CPU. The count is
// capped so that each worker's open directory fd, plus stdio, the
// runtime and one child at a time, fits RLIMIT_NOFILE (execdir_spec runs
// with rlimit_nofile 30). QFIND_WORKERS overrides it for benchmarking.
func (r *Run) numWorkers() int {
	if v, ok := r.lookupEnv("QFIND_WORKERS"); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	n := max(fastwalk.DefaultNumWorkers(), min(4*runtime.GOMAXPROCS(0), 32))
	var rl unix.Rlimit
	if unix.Getrlimit(unix.RLIMIT_NOFILE, &rl) == nil && rl.Cur != unix.RLIM_INFINITY {
		n = min(n, max(1, (int(rl.Cur)-16)/2))
	}
	return n
}

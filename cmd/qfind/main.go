// Command qfind is GNU find 4.11.0 reimplemented in Go on fastwalk.
package main

import (
	"os"
	"runtime/debug"
	"runtime/pprof"

	"qfind/internal/find"
)

func main() {
	// fastwalk alone allocates about 100 bytes per entry, and at the
	// default GOGC=100 the heap is collected every few MB: GOGC=400 walks
	// a warm 1M-file tree 20% faster for about 20 MB more memory. An
	// explicit GOGC in the environment still wins.
	if _, ok := os.LookupEnv("GOGC"); !ok {
		debug.SetGCPercent(400)
	}
	// QFIND_CPUPROFILE=file writes a CPU profile of the run; find stops it
	// through BeforeExit, because the process ends in os.Exit.
	if path := os.Getenv("QFIND_CPUPROFILE"); path != "" {
		if f, err := os.Create(path); err == nil && pprof.StartCPUProfile(f) == nil {
			find.BeforeExit = func() {
				pprof.StopCPUProfile()
				f.Close()
			}
		}
	}
	status := find.Main(os.Args)
	if find.BeforeExit != nil {
		find.BeforeExit()
	}
	os.Exit(status)
}

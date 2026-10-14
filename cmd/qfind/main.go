// Command qfind is GNU find 4.11.0 reimplemented in Go on fastwalk.
package main

import (
	"os"
	"runtime/pprof"

	"qfind/internal/find"
)

func main() {
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

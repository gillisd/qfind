// Package find is a port of GNU findutils 4.11.0 find/*.c with fts
// replaced by github.com/charlievieth/fastwalk.
package find

import (
	"fmt"
	"os"
)

// Main runs find with argv and returns the process exit status.
func Main(args []string) int {
	fmt.Fprintln(os.Stderr, "find: not implemented yet")
	return 1
}

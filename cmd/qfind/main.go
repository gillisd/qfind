// Command qfind is GNU find 4.11.0 reimplemented in Go on fastwalk.
package main

import (
	"os"

	"qfind/internal/find"
)

func main() {
	os.Exit(find.Main(os.Args))
}

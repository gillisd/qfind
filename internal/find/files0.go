package find

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// processAllStartPoints is ftsfind.c process_all_startpoints. An empty
// start point is reported and skipped, and makes the result false, so main
// skips the success rates and cleanup() as GNU does.
func (r *Run) processAllStartPoints() bool {
	if !r.opt.HasFiles0From {
		if len(r.startPoints) == 0 {
			return r.findRoot(".")
		}
		ok := true
		for _, name := range r.startPoints {
			if name == "" {
				r.errorf(unix.ENOENT, "%s", r.quote(name))
				r.setFailure()
				ok = false
				continue
			}
			if !r.findRoot(name) {
				return false
			}
		}
		return ok
	}
	if len(r.startPoints) > 0 {
		r.errorf(nil, "extra operand %s", r.quote(r.startPoints[0]))
		r.fatalf(nil, "file operands cannot be combined with -files0-from")
	}
	in, quoted := os.Stdin, r.quote("(standard input)")
	if r.opt.Files0From == "-" {
		if r.opt.OkPromptStdin {
			r.fatalf(nil, "when -ok or -okdir is in use, the option -files0-from cannot also read from standard input")
		}
	} else {
		quoted = r.quote(r.opt.Files0From)
		f, err := os.Open(r.opt.Files0From)
		if err != nil {
			r.fatalf(err, "cannot open %s for reading", quoted)
		}
		if r.opt.OkPromptStdin && sameInode(f, os.Stdin) {
			r.fatalf(nil, "when -ok or -okdir is in use, the option -files0-from cannot also read from standard input, but the file (%s) used for -files0-from appears to refer to the same file as the standard input", quoted)
		}
		in = f
	}
	if !r.findFiles0(bufio.NewReaderSize(in, 64<<10)) {
		return false
	}
	// GNU fcloses the stream, stdin included, so commands run by cleanup()
	// (pending -exec … +) start with fd 0 closed after -files0-from -.
	if in.Close() != nil {
		r.fatalf(nil, "error reading %s", r.slot0.load())
	}
	return true
}

// findFiles0 is the argv_iter loop over a NUL-separated stream: getdelim
// returns a last item without a terminating NUL, and data read before a
// failing read(2) is still an item, so the read error comes after it. Its
// messages name files0_filename_quoted, a pointer into quotearg slot 0.
func (r *Run) findFiles0(br *bufio.Reader) bool {
	ok := true
	for n := 1; ; n++ {
		item, err := br.ReadString(0)
		if item != "" {
			if item = strings.TrimSuffix(item, "\x00"); item == "" {
				r.errorf(nil, "%s:%d: %s", r.slot0.load(), n, "invalid zero-length file name")
				r.setFailure()
				ok = false
			} else if !r.findRoot(item) {
				return false
			}
		}
		switch {
		case errors.Is(err, io.EOF):
			return ok
		case err != nil:
			r.errorf(err, "%s: read error", r.slot0.load())
			r.setFailure()
			return false
		}
	}
}

func sameInode(a, b *os.File) bool {
	var sa, sb unix.Stat_t
	return unix.Fstat(int(a.Fd()), &sa) == nil && unix.Fstat(int(b.Fd()), &sb) == nil &&
		sa.Dev == sb.Dev && sa.Ino == sb.Ino
}

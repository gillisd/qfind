package find

import (
	"bufio"
	"os"
	"strings"
)

// askOk is pred.c is_ok + gnulib yesno: flush stdout, write
// "< cmd ... path > ? " raw (unquoted, the template's first word as typed)
// to stderr in one write, then read one line from find's buffered stdin
// (getline); EOF or an empty read means no. Both pred_ok and pred_okdir
// pass the full path (okdir_spec: "< echo ... tmp/fred > ? ").
// Evaluation is serialised when -ok/-okdir is present, so prompts and
// answers pair up one file at a time.
func (r *Run) askOk(e *Entry, ev *execVal) bool {
	r.flushStdout(e)
	if err := r.writeStderr("< " + ev.replaceVec[0] + " ... " + e.path + " > ? "); err != nil {
		r.fatalAt(e, err, "Failed to write prompt for -ok") // no cleanup: pending batches dropped
	}
	if r.stdin == nil {
		r.stdin = bufio.NewReader(os.Stdin)
	}
	line, err := r.stdin.ReadString('\n')
	if line == "" && err != nil {
		return false
	}
	return r.loc.IsYes(strings.TrimSuffix(line, "\n"))
}

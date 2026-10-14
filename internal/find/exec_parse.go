package find

import (
	"strings"
	"sync"
)

// execVal is defs.h struct exec_val.
type execVal struct {
	multiple, local, closeStdin bool
	replaceVec                  []string // ';' form: the command template
	ctl                         bcControl
	mu                          sync.Mutex
	batch                       bcState               // -exec +; also holds the initial arguments
	dirBatches                  map[batchKey]*bcState // -execdir +
	id                          FuncID
}

// batchKey groups -execdir … + arguments. GNU runs the pending command
// whenever fts changes level, so a batch never mixes directories or
// levels; "." and "/" share a working directory with their children but
// not a level.
type batchKey struct {
	dir   string
	depth int
}

// checkPathSafety is parser.c check_path_safety.
func (r *Run) checkPathSafety(action string) {
	path, ok := r.lookupEnv("PATH")
	if !ok {
		return
	}
	for _, f := range strings.Split(path, ":") {
		switch {
		case f == "" || f == ".":
			r.fatalf(nil, "The current directory is included in the PATH environment variable, which is insecure in combination with the %s action of find.  Please remove the current directory from your $PATH (that is, remove \".\", doubled colons, or leading or trailing colons)", action)
		case f[0] != '/':
			r.fatalf(nil, "The relative path %s is included in the PATH environment variable, which is insecure in combination with the %s action of find.  Please remove that entry from $PATH", r.quote(f), action)
		}
	}
}

// insertExecOk is parser.c insert_exec_ok.
func (ps *parser) insertExecOk(action string, e *parserEntry) bool {
	r := ps.r
	if ps.i >= len(ps.argv) {
		return false
	}
	p := ps.insertPrimaryWithPred(e, e.fn, e.id, "(some -exec* arguments)", true)
	p.sideEffects, p.noDefaultPrint = true, true
	p.needType, p.needStat = false, false
	ev := &execVal{id: e.id, local: e.id == fnExecdir || e.id == fnOkdir}
	p.args.exec = ev
	allowPlus := e.id == fnExec || e.id == fnExecdir
	if !allowPlus {
		r.opt.OkPromptStdin = true
		ev.closeStdin = true
	}
	if ev.local {
		r.opt.IgnoreReaddirRace = false
		r.checkPathSafety(action)
	}
	start, end := ps.i, ps.i
	prevBraces, braceCount := false, 0
	for ; end < len(ps.argv) && ps.argv[end] != ";"; end++ {
		a := ps.argv[end]
		if allowPlus && prevBraces && a == "+" {
			ev.multiple = true
			break
		}
		prevBraces = false
		if strings.Contains(a, "{}") {
			prevBraces = a == "{}"
			braceCount++
			if start == end && ev.local {
				r.fatalf(nil, "You may not use {} within the utility name for -execdir and -okdir, because this is a potential security problem.")
			}
		}
	}
	if end == start || end == len(ps.argv) {
		// GNU frees the node but leaves it linked as last_pred; the
		// command line is rejected right after, so nothing sees it.
		ps.i = end
		return false
	}
	// "+" ends the command only right after a lone "{}", which is then
	// brace_arg; so "must appear by itself" is unreachable (spec/README.md).
	if ev.multiple && braceCount > 1 {
		suffix := ""
		if e.id == fnExecdir {
			suffix = "dir"
		}
		r.fatalf(nil, "Only one instance of {} is supported with -exec%s ... +", suffix)
	}
	ctl, ok := bcInit(r.env)
	if !ok {
		r.fatalf(nil, "The environment is too large for exec().")
	}
	ev.ctl = ctl
	if ev.multiple {
		ev.ctl.initialArgc = end - start - 1
		for _, a := range ps.argv[start : end-1] {
			ev.ctl.push(r, nil, &ev.batch, a, "", true, false, nil)
		}
		ev.dirBatches = map[batchKey]*bcState{}
	} else {
		ev.replaceVec = append([]string(nil), ps.argv[start:end]...)
	}
	ps.i = end + 1
	return true
}

func parseExec(ps *parser, e *parserEntry) bool    { return ps.insertExecOk("-exec", e) }
func parseExecdir(ps *parser, e *parserEntry) bool { return ps.insertExecOk("-execdir", e) }
func parseOk(ps *parser, e *parserEntry) bool      { return ps.insertExecOk("-ok", e) }
func parseOkdir(ps *parser, e *parserEntry) bool   { return ps.insertExecOk("-okdir", e) }

package find

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"syscall"
)

var debugAssoc = []struct {
	name string
	val  DebugFlags
	doc  string
}{
	{"exec", DebugExec, "Show diagnostic information relating to -exec, -execdir, -ok and -okdir"},
	{"opt", DebugExpressionTree | DebugTreeOpt, "Show diagnostic information relating to optimisation"},
	{"rates", DebugSuccessRates, "Indicate how often each predicate succeeded"},
	{"search", DebugSearch, "Navigate the directory tree verbosely"},
	{"stat", DebugStat, "Trace calls to stat(2) and lstat(2)"},
	{"time", DebugTime, "Show diagnostic information relating to time-of-day and timestamp comparisons"},
	{"tree", DebugExpressionTree, "Display the expression tree"},
	{"all", DebugAll, "Set all of the debug flags (but help)"},
	{"help", DebugHelp, "Explain the various -D options"},
}

// processLeadingOptions is util.c process_leading_options; it returns the
// index of the first argument after the leading options.
func (r *Run) processLeadingOptions(args []string) int {
	i := 1
	for ; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-H":
			r.setFollowState(SymlinkDerefArgsOnly)
		case a == "-L":
			r.setFollowState(SymlinkAlwaysDeref)
		case a == "-P":
			r.setFollowState(SymlinkNeverDeref)
		case a == "--":
			return i + 1
		case a == "-D":
			if i+1 >= len(args) {
				r.errorf(nil, "Missing argument after the -D option.")
				r.usageFailure()
			}
			r.processDebugOptions(args[i+1])
			i++
		case strings.HasPrefix(a, "-O"):
			r.processOptimisationOption(a[2:])
		default:
			return i
		}
	}
	return i
}

// processDebugOptions is util.c process_debug_options. The unknown-flag
// warning quotes arg as strtok_r has left it, cut after the FIRST token
// (leading commas included) rather than the unknown flag, which is a
// 4.11.0 bug the spec tolerates; reproduce it.
func (r *Run) processDebugOptions(arg string) {
	tokens := strings.FieldsFunc(arg, func(c rune) bool { return c == ',' })
	if len(tokens) == 0 {
		r.errorf(nil, "Empty argument to the -D option.")
		r.usageFailure()
	}
	leadingCommas := len(arg) - len(strings.TrimLeft(arg, ","))
	cut := arg[:leadingCommas+len(tokens[0])]
	for _, t := range tokens {
		found := false
		for _, d := range debugAssoc {
			if d.name == t {
				r.opt.Debug |= d.val
				found = true
				break
			}
		}
		if !found {
			r.errorf(nil, "Ignoring unrecognised debug flag %s", r.quote(cut))
		}
	}
	if r.opt.Debug&DebugHelp != 0 {
		r.showValidDebugOptions(true)
		r.exit(0)
	}
}

// showValidDebugOptions is util.c show_valid_debug_options.
func (r *Run) showValidDebugOptions(full bool) {
	var b strings.Builder
	b.WriteString("Valid arguments for -D:\n")
	for i, d := range debugAssoc {
		if full {
			fmt.Fprintf(&b, "%-10s %s\n", d.name, d.doc)
		} else if i == 0 {
			b.WriteString("      " + d.name)
		} else {
			b.WriteString(", " + d.name)
		}
	}
	r.out.WriteString(b.String())
}

// processOptimisationOption is util.c process_optimisation_option.
func (r *Run) processOptimisationOption(arg string) {
	if arg == "" {
		r.fatalf(nil, "The -O option must be immediately followed by a decimal integer")
	}
	if arg[0] < '0' || arg[0] > '9' {
		r.fatalf(nil, "Please specify a decimal number immediately after -O")
	}
	end := 0
	for end < len(arg) && arg[end] >= '0' && arg[end] <= '9' {
		end++
	}
	if end < len(arg) {
		r.fatalf(nil, "Invalid optimisation level %s", arg)
	}
	level, err := strconv.ParseUint(arg, 10, 64)
	if errors.Is(err, strconv.ErrRange) {
		r.fatalf(syscall.ERANGE, "Invalid optimisation level %s", arg)
	}
	if level > 65535 {
		r.fatalf(nil, "Optimisation level %d is too high.  If you want to find files very quickly, consider using GNU locate.", level)
	}
	r.opt.OptimisationLevel = int(level)
}

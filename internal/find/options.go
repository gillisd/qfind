package find

import (
	"time"

	"golang.org/x/sys/unix"

	"qfind/internal/regex"
)

// SymlinkMode is enum SymlinkOption (defs.h).
type SymlinkMode uint8

const (
	SymlinkNeverDeref    SymlinkMode = iota // -P (default)
	SymlinkDerefArgsOnly                    // -H
	SymlinkAlwaysDeref                      // -L, -follow
)

// DebugFlags are the -D bits (defs.h DebugXxx values).
type DebugFlags uint

const (
	DebugExpressionTree DebugFlags = 1 << iota // tree
	DebugStat                                  // stat
	DebugSearch                                // search
	DebugTreeOpt                               // opt (with tree)
	DebugHelp                                  // help
	DebugExec                                  // exec
	DebugSuccessRates                          // rates
	DebugTime                                  // time
	DebugAll            = DebugExpressionTree | DebugStat | DebugSearch | DebugTreeOpt |
		DebugExec | DebugSuccessRates | DebugTime
)

// Timespec mirrors struct timespec (nanosecond timestamps compare exactly).
type Timespec struct{ Sec, Nsec int64 }

// Options is struct options (defs.h).
type Options struct {
	DoDirFirst          bool
	ExplicitDepth       bool
	MaxDepth, MinDepth  int // -1 when unset
	NoLeafCheck         bool
	Mount, Xdev         bool
	IgnoreReaddirRace   bool
	LiteralControlChars bool
	Warnings            bool
	PosixlyCorrect      bool
	StartTime           Timespec
	CurDayStart         Timespec
	FullDays            bool
	OutputBlockSize     int
	Debug               DebugFlags
	Symlinks            SymlinkMode
	RegexSyntax         regex.Syntax // -regextype; GNU defaults to RE_SYNTAX_EMACS, not findutils-default
	OptimisationLevel   int
	Files0From          string
	HasFiles0From       bool
	OkPromptStdin       bool
}

// setDefaults is util.c set_option_defaults.
func (r *Run) setDefaults() {
	o := &r.opt
	_, o.PosixlyCorrect = r.lookupEnv("POSIXLY_CORRECT")
	o.RegexSyntax = regex.SyntaxEmacs
	o.Warnings = isatty(0) && !o.PosixlyCorrect
	o.DoDirFirst = true
	o.MaxDepth, o.MinDepth = -1, -1
	now := time.Now()
	o.StartTime = Timespec{now.Unix(), int64(now.Nanosecond())}
	o.CurDayStart = Timespec{o.StartTime.Sec - 86400, o.StartTime.Nsec}
	o.OutputBlockSize = 1024
	if o.PosixlyCorrect {
		o.OutputBlockSize = 512
	}
	o.OptimisationLevel = 1
	if _, ok := r.lookupEnv("FIND_BLOCK_SIZE"); ok {
		r.fatalf(nil, "The environment variable FIND_BLOCK_SIZE is not supported, the only thing that affects the block size is the POSIXLY_CORRECT environment variable")
	}
	o.NoLeafCheck = false // LEAF_OPTIMISATION is on in the reference build
	r.setFollowState(SymlinkNeverDeref)
}

// setFollowState is parser.c set_follow_state.
func (r *Run) setFollowState(m SymlinkMode) {
	r.opt.Symlinks = m
	if m != SymlinkNeverDeref {
		r.opt.NoLeafCheck = true
	}
}

func isatty(fd int) bool {
	_, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	return err == nil
}

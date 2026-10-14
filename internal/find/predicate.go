package find

import (
	"sync/atomic"

	"qfind/internal/regex"
)

// PredType is enum predicate_type (defs.h).
type PredType uint8

const (
	NoType PredType = iota
	PrimaryType
	UniOp
	BiOp
	OpenParen
	CloseParen
)

// Prec is enum predicate_precedence (defs.h).
type Prec uint8

const (
	NoPrec Prec = iota
	CommaPrec
	OrPrec
	AndPrec
	NegatePrec
	MaxPrec
)

// Cost is enum EvaluationCost (defs.h). The order matters: the optimiser
// compares costs and buckets predicates by them.
type Cost uint8

const (
	NeedsNothing Cost = iota
	NeedsInodeNumber
	NeedsType
	NeedsStatInfo
	NeedsLinkName
	NeedsAccessInfo
	NeedsSyncDiskHit
	NeedsEventualExec
	NeedsImmediateExec
	NeedsUserInteraction
	NeedsUnknown
	numCosts
)

// FuncID names the pred_* function of a node. GNU compares pred_func
// pointers (pred_is, the cost table); Go func values are not comparable.
type FuncID uint8

const (
	fnNone FuncID = iota
	fnAmin
	fnAnd
	fnAnewer
	fnAtime
	fnCloseParen
	fnCmin
	fnCnewer
	fnComma
	fnContext
	fnCtime
	fnDelete
	fnEmpty
	fnExec
	fnExecdir
	fnExecutable
	fnFalse
	fnFls
	fnFprint
	fnFprint0
	fnFprintf
	fnFstype
	fnGid
	fnGroup
	fnIlname
	fnIname
	fnInum
	fnIpath
	fnLinks
	fnLname
	fnLs
	fnMmin
	fnMtime
	fnName
	fnNegate
	fnNewer
	fnNewerXY
	fnNogroup
	fnNouser
	fnOk
	fnOkdir
	fnOpenParen
	fnOr
	fnPath
	fnPerm
	fnPrint
	fnPrint0
	fnPrune
	fnQuit
	fnReadable
	fnRegex
	fnSamefile
	fnSize
	fnTrue
	fnType
	fnUid
	fnUsed
	fnUser
	fnWritable
	fnXtype
)

// PredFunc is a pred_* function.
type PredFunc func(r *Run, e *Entry, p *Predicate) bool

// Predicate is struct predicate (defs.h). next links command-line order;
// left/right form the evaluation tree.
type Predicate struct {
	fn             PredFunc
	id             FuncID
	name           string // p_name, as typed
	typ            PredType
	prec           Prec
	sideEffects    bool
	noDefaultPrint bool
	needStat       bool
	needType       bool
	needInum       bool
	cost           Cost
	estSuccessRate float32
	artificial     bool
	argText        string
	hasArgText     bool
	args           predArgs
	next           *Predicate
	left, right    *Predicate
	entry          *parserEntry
	visits         atomic.Uint64 // perf.visits
	successes      atomic.Uint64 // perf.successes
}

// predArgs is the args union of struct predicate. Tasks add fields as they
// port predicates (regex, exec, num, size, uid, gid, reftime, perm,
// samefile, types, format, scontext).
type predArgs struct {
	str    string           // fstype, [i]lname, [i]name, [i]path
	out    *Stream          // printf_vec.stream for the print family
	format *formatVal       // printf_vec for -printf and -fprintf
	types  [ftypeCount]bool // -type, -xtype
	regex  *regex.Regexp    // -regex, -iregex
}

// is is defs.h pred_is.
func (p *Predicate) is(id FuncID) bool { return p.id == id }

// apply is util.c apply_predicate. GNU counts visits and successes always
// but shows them only under -D rates, so qfind counts only then.
func (r *Run) apply(e *Entry, p *Predicate) bool {
	if r.countRates {
		p.visits.Add(1)
	}
	if p.needStat || p.needType || p.needInum {
		if !r.getInfo(e, p) {
			return false
		}
	}
	if !p.fn(r, e, p) {
		return false
	}
	if r.countRates {
		p.successes.Add(1)
	}
	return true
}

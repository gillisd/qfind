// Package regex implements glibc's re_compile_pattern/re_match for the
// GNU syntaxes selectable with find -regextype.
package regex

// Syntax is reg_syntax_t: the RE_* bits of gl/lib/regex.h.
type Syntax uint32

const (
	BackslashEscapeInLists Syntax = 1 << iota
	BkPlusQM
	CharClasses
	ContextIndepAnchors
	ContextIndepOps
	ContextInvalidOps
	DotNewline
	DotNotNull
	HatListsNotNewline
	Intervals
	LimitedOps
	NewlineAlt
	NoBkBraces
	NoBkParens
	NoBkRefs
	NoBkVbar
	NoEmptyRanges
	UnmatchedRightParenOrd
	NoPosixBacktracking
	NoGNUOps
	Debug
	InvalidIntervalOrd
	ICase
	CaretAnchorsHere
	ContextInvalidDup
	NoSub
)

// The RE_SYNTAX_* combinations of gnulib's regex.h, which findutils
// compiles against (RE_SYNTAX_EMACS is 0 in glibc's own header).
const (
	SyntaxEmacs             = CharClasses | Intervals
	SyntaxAwk               = BackslashEscapeInLists | DotNotNull | NoBkParens | NoBkRefs | NoBkVbar | NoEmptyRanges | DotNewline | ContextIndepAnchors | CharClasses | UnmatchedRightParenOrd | NoGNUOps
	posixCommon             = CharClasses | DotNewline | DotNotNull | Intervals | NoEmptyRanges
	SyntaxPosixBasic        = posixCommon | BkPlusQM | ContextInvalidDup
	SyntaxPosixMinimalBasic = posixCommon | LimitedOps
	SyntaxPosixExtended     = posixCommon | ContextIndepAnchors | ContextIndepOps | NoBkBraces | NoBkParens | NoBkVbar | ContextInvalidOps | UnmatchedRightParenOrd
	SyntaxGNUAwk            = (SyntaxPosixExtended | BackslashEscapeInLists | InvalidIntervalOrd) &^ (DotNotNull | ContextIndepOps | ContextInvalidOps)
	SyntaxPosixAwk          = SyntaxPosixExtended | BackslashEscapeInLists | Intervals | NoGNUOps | InvalidIntervalOrd
	SyntaxGrep              = (SyntaxPosixBasic | NewlineAlt) &^ (ContextInvalidDup | DotNotNull)
	SyntaxEgrep             = (SyntaxPosixExtended | InvalidIntervalOrd | NewlineAlt) &^ (ContextInvalidOps | DotNotNull)
)

// RE_DUP_MAX is the largest interval count.
const dupMax = 0x7fff

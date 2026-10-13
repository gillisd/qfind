// Package regex implements GNU regex syntaxes (Task 10 completes it).
package regex

// Syntax is a set of RE_* syntax bits (gnulib regex.h).
type Syntax uint32

// SyntaxEmacs is RE_SYNTAX_EMACS = RE_CHAR_CLASSES | RE_INTERVALS.
const SyntaxEmacs Syntax = 1<<2 | 1<<9 // RE_CHAR_CLASSES | RE_INTERVALS (regex.h: bits 2 and 9)

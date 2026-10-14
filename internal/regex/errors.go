package regex

// errcode is reg_errcode_t (gl/lib/regex.h).
type errcode int

const (
	regNoError errcode = iota
	regNoMatch
	regBadPat
	regECollate
	regECtype
	regEEscape
	regESubreg
	regEBrack
	regEParen
	regEBrace
	regBadBR
	regERange
	regESpace
	regBadRpt
	regEEnd
	regESize
	regERParen
)

// messages is regcomp.c __re_error_msgid, as re_compile_pattern returns it.
var messages = [...]string{"Success", "No match", "Invalid regular expression",
	"Invalid collation character", "Invalid character class name", "Trailing backslash",
	"Invalid back reference", "Unmatched [, [^, [:, [., or [=", "Unmatched ( or \\(",
	"Unmatched \\{", "Invalid content of \\{\\}", "Invalid range end", "Memory exhausted",
	"Invalid preceding regular expression", "Premature end of regular expression",
	"Regular expression too big", "Unmatched ) or \\)"}

// Error is a compile error whose text is glibc's re_compile_pattern message.
type Error struct{ code errcode }

func (e *Error) Error() string { return messages[e.code] }

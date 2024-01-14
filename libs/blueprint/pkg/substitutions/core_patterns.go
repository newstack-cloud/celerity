package substitutions

import "regexp"

// There are no patterns in this file for string literals
// or function calls as they are complex to represent with
// regular expressions that do not support lookaheads.
// They are instead handled by a hand-rolled
// sequential character processing state machine.
// See seq_parser.go for more details.
//
// These patterns are used to efficiently check if a full
// substitution string is an exact match for a literal.

var (
	// IntLiteralPattern is the pattern for an integer
	// that can be used in a substitution.
	// Only decimal representations are supported. (As opposed to hex, octal, etc.)
	IntLiteralPattern = regexp.MustCompile(
		`^-?\d+$`,
	)

	// FloatLiteralPattern is the pattern for a floating point
	// number that can be used in a substitution.
	// Only decimal representations are supported. (As opposed to hex, octal, etc.)
	FloatLiteralPattern = regexp.MustCompile(
		`^-?\d+(\.\d+)?$`,
	)

	// BoolLiteralPattern is the pattern for a boolean
	// that can be used in a substitution.
	BoolLiteralPattern = regexp.MustCompile(
		`^(true|false)$`,
	)
)

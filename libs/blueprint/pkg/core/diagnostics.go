package core

import (
	"github.com/two-hundred/celerity/libs/blueprint/pkg/source"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/substitutions"
)

// Diagnostic provides a warning or informational diagnostic for a blueprint.
type Diagnostic struct {
	// The level of this diagnostic.
	Level DiagnosticLevel
	// The message of this diagnostic.
	Message string
	// An optional text range in the source blueprint
	// that the diagnostic applies to.
	// This will only be present when the source format is YAML,
	// but can be nil for some diagnostics from a YAML source input.
	Range *DiagnosticRange
}

// DiagnosticLevel provides the level of a diagnostic.
type DiagnosticLevel int

const (
	// DiagnosticLevelError should be used for diagnostics that point out
	// errors in a blueprint.
	// This is not solely used as a source of errors, this should be combined
	// with unpacking a returned error to produce a set of error diagnostics
	// for tools that report diagnostics. (e.g. language servers)
	DiagnosticLevelError DiagnosticLevel = 1
	// DiagnosticLevelWarning should be used for diagnostics that point out
	// potential issues that may occur when executing a blueprint.
	DiagnosticLevelWarning DiagnosticLevel = 2
	// DiagnosticLevelInfo should be used for diagnostics that provide
	// informational messages about the blueprint that are worth noting
	// but do not indicate issues that may occur when executing a blueprint.
	DiagnosticLevelInfo DiagnosticLevel = 3
)

// DiagnosticRange provides a range in the source blueprint that a diagnostic applies to.
// This will only be used for source formats that allow position tracking of parsed nodes
// (i.e. YAML source documents).
type DiagnosticRange struct {
	Start *source.Meta
	End   *source.Meta
	// When the diagnostic is concerning contents of a ${..} substitution,
	// depending on the context, the column may not be accurate,
	// this gives you the option to ignore approximate columns in contexts
	// where they are likely to cause confusion for the end-user.
	// (e.g. language server diagnostics for a code editor)
	ColumnAccuracy *substitutions.ColumnAccuracy
}

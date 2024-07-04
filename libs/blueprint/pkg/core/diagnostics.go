package core

import "github.com/two-hundred/celerity/libs/blueprint/pkg/source"

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
}

package source

import (
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Meta represents information about the deserialised source of
// a blueprint value including the line and column
// where a blueprint element begins that can be used by tools such
// as linters to provide more detailed diagnostics to users creating
// blueprints from source in some supported formats.
type Meta struct {
	Position
	EndPosition *Position `json:"endPosition,omitempty"`
}

// Position represents a position in the source code of a blueprint.
type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// PositionFromSourceMeta returns the line and column from the provided source meta.
// This is primarily useful for attaching position information to errors.
func PositionFromSourceMeta(sourceMeta *Meta) (line *int, column *int) {
	if sourceMeta == nil {
		return nil, nil
	}

	return &sourceMeta.Line, &sourceMeta.Column
}

// EndSourcePositionFromYAMLScalarNode returns the precise
// end position of a YAML scalar node.
func EndSourcePositionFromYAMLScalarNode(node *yaml.Node) *Position {
	if node.Kind != yaml.ScalarNode {
		return nil
	}

	if node.Style == yaml.DoubleQuotedStyle || node.Style == yaml.SingleQuotedStyle {
		charCount := utf8.RuneCountInString(node.Value) + 2
		return &Position{
			Line:   node.Line,
			Column: node.Column + charCount,
		}
	}

	// 0 indicates plain style
	if node.Style == 0 {
		return &Position{
			Line:   node.Line,
			Column: node.Column + utf8.RuneCountInString(node.Value),
		}
	}

	lines := strings.Split(strings.ReplaceAll(node.Value, "\r\n", "\n"), "\n")
	lineCountInBlock := len(lines) - 1
	columnOnLastLine := node.Column

	if lineCountInBlock > 0 {
		columnOnLastLine += utf8.RuneCountInString(lines[lineCountInBlock-1])
	}

	return &Position{
		Line:   node.Line + lineCountInBlock,
		Column: columnOnLastLine - 1,
	}
}

// Range represents a range in the source code of a blueprint.
// Start and End could also hold additional information provided
// in the Meta struct.
type Range struct {
	Start *Position
	End   *Position
}

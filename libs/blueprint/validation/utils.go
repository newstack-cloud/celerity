package validation

import (
	"fmt"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
)

func deriveVarType(value *bpcore.ScalarValue) schema.VariableType {
	if value != nil && value.IntValue != nil {
		return schema.VariableTypeInteger
	}

	if value != nil && value.FloatValue != nil {
		return schema.VariableTypeFloat
	}

	if value != nil && value.BoolValue != nil {
		return schema.VariableTypeBoolean
	}

	// This should only ever be used in a context where
	// the given scalar has a value, so string will always
	// be the default.
	return schema.VariableTypeString
}

func deriveScalarValueAsString(value *bpcore.ScalarValue) string {
	if value != nil && value.IntValue != nil {
		return fmt.Sprintf("%d", *value.IntValue)
	}

	if value != nil && value.FloatValue != nil {
		return fmt.Sprintf("%.2f", *value.FloatValue)
	}

	if value != nil && value.BoolValue != nil {
		return fmt.Sprintf("%t", *value.BoolValue)
	}

	if value != nil && value.StringValue != nil {
		return *value.StringValue
	}

	return ""
}

func varTypeToUnit(varType schema.VariableType) string {
	switch varType {
	case schema.VariableTypeInteger:
		return "an integer"
	case schema.VariableTypeFloat:
		return "a float"
	case schema.VariableTypeBoolean:
		return "a boolean"
	case schema.VariableTypeString:
		return "a string"
	default:
		return "an unknown type"
	}
}

func toDiagnosticRange(
	start *source.Meta,
	nextLocation *source.Meta,
) *bpcore.DiagnosticRange {
	if start == nil {
		return &bpcore.DiagnosticRange{
			Start: &source.Meta{
				Line:   1,
				Column: 1,
			},
			End: &source.Meta{
				Line:   1,
				Column: 1,
			},
		}
	}

	endSourceMeta := &source.Meta{
		Line:   start.Line + 1,
		Column: 1,
	}
	if nextLocation != nil {
		endSourceMeta = &source.Meta{
			Line:   nextLocation.Line,
			Column: nextLocation.Column,
		}
	}

	return &bpcore.DiagnosticRange{
		Start: start,
		End:   endSourceMeta,
	}
}

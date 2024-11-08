package includes

import "github.com/two-hundred/celerity/libs/blueprint/core"

// StringValue extracts the string value from a MappingNode.
func StringValue(value *core.MappingNode) string {
	if value == nil || value.Literal == nil || value.Literal.StringValue == nil {
		return ""
	}

	return *value.Literal.StringValue
}

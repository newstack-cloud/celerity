package core

import (
	"fmt"
	"strings"
)

// Sum calculates the sum of a list of numbers.
func Sum(numbers []float64) float64 {
	sum := 0.0
	for _, number := range numbers {
		sum += number
	}
	return sum
}

// IsInScalarList checks if a given scalar value is in a list of scalar values.
func IsInScalarList(value *ScalarValue, list []*ScalarValue) bool {
	found := false
	i := 0
	for !found && i < len(list) {
		found = list[i].Equal(value)
		i += 1
	}
	return found
}

// Flatten returns a flattened 2D array of the given type.
func Flatten[Item any](array [][]Item) []Item {
	flattened := []Item{}
	for _, row := range array {
		flattened = append(flattened, row...)
	}
	return flattened
}

// StringValue extracts the string value from a MappingNode.
func StringValue(value *MappingNode) string {
	if value == nil || value.Scalar == nil || value.Scalar.StringValue == nil {
		return ""
	}

	return *value.Scalar.StringValue
}

// BoolValue extracts the boolean value from a MappingNode.
// This will return false if the value is nil or the given mapping node
// contains another type of value.
func BoolValue(value *MappingNode) bool {
	if value == nil || value.Scalar == nil || value.Scalar.BoolValue == nil {
		return false
	}

	return *value.Scalar.BoolValue
}

// IntValue extracts the integer value from a MappingNode.
// This will return 0 if the value is nil or the given mapping node
// contains another type of value.
func IntValue(value *MappingNode) int {
	if value == nil || value.Scalar == nil || value.Scalar.IntValue == nil {
		return 0
	}

	return *value.Scalar.IntValue
}

// FloatValue extracts the float value from a MappingNode.
// This will return 0.0 if the value is nil or the given mapping node
// contains another type of value.
func FloatValue(value *MappingNode) float64 {
	if value == nil || value.Scalar == nil || value.Scalar.FloatValue == nil {
		return 0.0
	}

	return *value.Scalar.FloatValue
}

// MappingNodeFromString creates a MappingNode from a string value.
func MappingNodeFromString(value string) *MappingNode {
	return &MappingNode{
		Scalar: &ScalarValue{
			StringValue: &value,
		},
	}
}

// MappingNodeFromBool creates a MappingNode from a boolean value.
func MappingNodeFromBool(value bool) *MappingNode {
	return &MappingNode{
		Scalar: &ScalarValue{
			BoolValue: &value,
		},
	}
}

// MappingNodeFromInt creates a MappingNode from an integer value.
func MappingNodeFromInt(value int) *MappingNode {
	return &MappingNode{
		Scalar: &ScalarValue{
			IntValue: &value,
		},
	}
}

// MappingNodeFromFloat creates a MappingNode from a float value.
func MappingNodeFromFloat(value float64) *MappingNode {
	return &MappingNode{
		Scalar: &ScalarValue{
			FloatValue: &value,
		},
	}
}

// ResourceElementID generates an element ID for a resource that is used
// primarily for resolving substitutions.
func ResourceElementID(resourceName string) string {
	return fmt.Sprintf("resources.%s", resourceName)
}

// VariableElementID generates an element ID for a variable that is used
// primarily for resolving substitutions.
func VariableElementID(variableName string) string {
	return fmt.Sprintf("variables.%s", variableName)
}

// ValueElementID generates an element ID for a value that is used
// primarily for resolving substitutions.
func ValueElementID(valueName string) string {
	return fmt.Sprintf("values.%s", valueName)
}

// ChildElementID generates an element ID for a child blueprint that is used
// primarily for resolving substitutions.
func ChildElementID(childName string) string {
	return fmt.Sprintf("children.%s", childName)
}

// DataSourceElementID generates an element ID for a data source that is used
// primarily for resolving substitutions.
func DataSourceElementID(dataSourceName string) string {
	return fmt.Sprintf("datasources.%s", dataSourceName)
}

// ExportElementID generates an element ID for a blueprint export that is used
// primarily for resolving substitutions.
func ExportElementID(dataSourceName string) string {
	return fmt.Sprintf("exports.%s", dataSourceName)
}

// ElementPropertyPath generates a property path for a given element ID and property name.
func ElementPropertyPath(elementID string, propertyName string) string {
	if strings.HasPrefix(propertyName, "[") {
		return fmt.Sprintf("%s%s", elementID, propertyName)
	}
	return fmt.Sprintf("%s.%s", elementID, propertyName)
}

// ExpandedResourceName generates a resource name with an index appended to it
// for resources expanded from a resource template.
func ExpandedResourceName(resourceTemplateName string, index int) string {
	return fmt.Sprintf("%s_%d", resourceTemplateName, index)
}

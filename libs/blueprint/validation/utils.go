package validation

import (
	"context"
	"fmt"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
)

func deriveVarType(value *core.ScalarValue) schema.VariableType {
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

func deriveScalarValueAsString(value *core.ScalarValue) string {
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
) *core.DiagnosticRange {
	if start == nil {
		return &core.DiagnosticRange{
			Start: &source.Meta{Position: source.Position{
				Line:   1,
				Column: 1,
			}},
			End: &source.Meta{Position: source.Position{
				Line:   1,
				Column: 1,
			}},
		}
	}

	endSourceMeta := &source.Meta{Position: source.Position{
		Line:   start.Line + 1,
		Column: 1,
	}}
	if nextLocation != nil {
		endSourceMeta = &source.Meta{Position: source.Position{
			Line:   nextLocation.Line,
			Column: nextLocation.Column,
		}}
	}

	return &core.DiagnosticRange{
		Start: start,
		End:   endSourceMeta,
	}
}

func isSubPrimitiveType(subType string) bool {
	switch substitutions.ResolvedSubExprType(subType) {
	case substitutions.ResolvedSubExprTypeString,
		substitutions.ResolvedSubExprTypeInteger,
		substitutions.ResolvedSubExprTypeFloat,
		substitutions.ResolvedSubExprTypeBoolean:
		return true
	default:
		return false
	}
}

func isEmptyStringWithSubstitutions(stringWithSubs *substitutions.StringOrSubstitutions) bool {
	if stringWithSubs == nil || stringWithSubs.Values == nil {
		return true
	}

	i := 0
	hasContent := false
	for !hasContent && i < len(stringWithSubs.Values) {
		if stringWithSubs.Values[i].SubstitutionValue != nil {
			hasContent = true
		} else {
			strVal := stringWithSubs.Values[i].StringValue
			hasContent = strVal != nil && strings.TrimSpace(*strVal) != ""
		}
		i += 1
	}

	return !hasContent
}

func validateDescription(
	ctx context.Context,
	usedIn string,
	description *substitutions.StringOrSubstitutions,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if description == nil {
		return diagnostics, nil
	}

	errs := []error{}

	for _, stringOrSub := range description.Values {
		if stringOrSub.SubstitutionValue != nil {
			resolvedType, subDiagnostics, err := ValidateSubstitution(
				ctx,
				stringOrSub.SubstitutionValue,
				nil,
				bpSchema,
				usedIn,
				"description",
				params,
				funcRegistry,
				refChainCollector,
				resourceRegistry,
			)
			if err != nil {
				errs = append(errs, err)
			} else {
				diagnostics = append(diagnostics, subDiagnostics...)
				if !isSubPrimitiveType(resolvedType) {
					errs = append(errs, errInvalidDescriptionSubType(
						usedIn,
						resolvedType,
						stringOrSub.SourceMeta,
					))
				}
			}
		}
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func getSubNextLocation(i int, values []*substitutions.StringOrSubstitution) *source.Meta {
	if i+1 < len(values) {
		return values[i+1].SourceMeta
	}

	return nil
}

func getEndLocation(location *source.Meta) *source.Meta {
	if location == nil {
		return nil
	}

	return &source.Meta{Position: source.Position{
		Line:   location.Line + 1,
		Column: location.Column,
	}}
}

func isMappingNodeEmpty(node *core.MappingNode) bool {
	return node == nil || (node.Literal == nil && node.Fields == nil &&
		node.Items == nil && node.StringWithSubstitutions == nil)
}

func deriveMappingNodeResourceDefinitionsType(node *core.MappingNode) provider.ResourceDefinitionsSchemaType {
	if node.Literal != nil && node.Literal.BoolValue != nil {
		return provider.ResourceDefinitionsSchemaTypeBoolean
	}

	if node.Literal != nil && node.Literal.StringValue != nil {
		return provider.ResourceDefinitionsSchemaTypeString
	}

	if node.Literal != nil && node.Literal.IntValue != nil {
		return provider.ResourceDefinitionsSchemaTypeInteger
	}

	if node.Literal != nil && node.Literal.FloatValue != nil {
		return provider.ResourceDefinitionsSchemaTypeFloat
	}

	if node.Fields != nil {
		return provider.ResourceDefinitionsSchemaTypeObject
	}

	if node.Items != nil {
		return provider.ResourceDefinitionsSchemaTypeArray
	}

	if node.StringWithSubstitutions != nil {
		return provider.ResourceDefinitionsSchemaTypeString
	}

	return ""
}

func resourceDefinitionsUnionTypeToString(unionSchema []*provider.ResourceDefinitionsSchema) string {
	var sb strings.Builder
	sb.WriteString("(")
	for i, schema := range unionSchema {
		sb.WriteString(string(schema.Type))
		if i < len(unionSchema)-1 {
			sb.WriteString(" | ")
		}
	}
	sb.WriteString(")")
	return sb.String()
}

// CreateSubRefTag creates a reference chain node tag for a substitution reference.
func CreateSubRefTag(usedIn string) string {
	return fmt.Sprintf("subRef:%s", usedIn)
}

// CreateSubRefPropTag creates a reference chain node tag for a substitution reference
// including the property path within the resource that holds the reference.
func CreateSubRefPropTag(usedIn string, usedInPropPath string) string {
	return fmt.Sprintf("subRefProp:%s:%s", usedIn, usedInPropPath)
}

// CreateDependencyRefTag creates a reference chain node tag for a dependency reference
// defined in a blueprint resource with the "dependsOn" property.
func CreateDependencyRefTag(usedIn string) string {
	return fmt.Sprintf("dependencyOf:%s", usedIn)
}

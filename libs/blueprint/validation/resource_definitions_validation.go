package validation

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/refgraph"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
)

func validateResourceDefinition(
	ctx context.Context,
	resourceName string,
	resourceType string,
	resourceDerivedFromTemplate bool,
	spec *core.MappingNode,
	parentLocation *source.Meta,
	validateAgainstSchema *provider.ResourceDefinitionsSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector refgraph.RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
	path string,
	depth int,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}
	// Counting depth starts from 0.
	if depth >= core.MappingNodeMaxTraverseDepth {
		return diagnostics, nil
	}

	isEmpty := isMappingNodeEmpty(spec)
	if isEmpty && validateAgainstSchema.Nullable {
		return diagnostics, nil
	}

	if validateAgainstSchema.Computed {
		return diagnostics, errComputedFieldDefinedInBlueprint(
			path,
			resourceName,
			selectMappingNodeLocation(spec, parentLocation),
		)
	}

	switch validateAgainstSchema.Type {
	case provider.ResourceDefinitionsSchemaTypeObject:
		return validateResourceDefinitionObject(
			ctx,
			resourceName,
			resourceType,
			resourceDerivedFromTemplate,
			spec,
			parentLocation,
			validateAgainstSchema,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			path,
			depth,
		)
	case provider.ResourceDefinitionsSchemaTypeMap:
		return validateResourceDefinitionMap(
			ctx,
			resourceName,
			resourceType,
			resourceDerivedFromTemplate,
			spec,
			parentLocation,
			validateAgainstSchema,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			path,
			depth,
		)
	case provider.ResourceDefinitionsSchemaTypeArray:
		return validateResourceDefinitionArray(
			ctx,
			resourceName,
			resourceType,
			resourceDerivedFromTemplate,
			spec,
			parentLocation,
			validateAgainstSchema,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			path,
			depth,
		)
	case provider.ResourceDefinitionsSchemaTypeString:
		return validateResourceDefinitionString(
			ctx,
			resourceName,
			resourceDerivedFromTemplate,
			spec,
			parentLocation,
			validateAgainstSchema,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			path,
		)
	case provider.ResourceDefinitionsSchemaTypeInteger:
		return validateResourceDefinitionInteger(
			ctx,
			resourceName,
			resourceDerivedFromTemplate,
			spec,
			parentLocation,
			validateAgainstSchema,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			path,
		)
	case provider.ResourceDefinitionsSchemaTypeFloat:
		return validateResourceDefinitionFloat(
			ctx,
			resourceName,
			resourceDerivedFromTemplate,
			spec,
			parentLocation,
			validateAgainstSchema,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			path,
		)
	case provider.ResourceDefinitionsSchemaTypeBoolean:
		return validateResourceDefinitionBoolean(
			ctx,
			resourceName,
			resourceDerivedFromTemplate,
			spec,
			parentLocation,
			validateAgainstSchema,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			path,
		)
	case provider.ResourceDefinitionsSchemaTypeUnion:
		return validateResourceDefinitionUnion(
			ctx,
			resourceName,
			resourceType,
			resourceDerivedFromTemplate,
			spec,
			parentLocation,
			validateAgainstSchema,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			path,
			depth,
		)
	default:
		return diagnostics, provider.ErrUnknownResourceDefSchemaType(
			validateAgainstSchema.Type,
			resourceType,
		)
	}
}

func validateResourceDefinitionObject(
	ctx context.Context,
	resourceName string,
	resourceType string,
	resourceDerivedFromTemplate bool,
	node *core.MappingNode,
	parentLocation *source.Meta,
	validateAgainstSchema *provider.ResourceDefinitionsSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector refgraph.RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
	path string,
	depth int,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(node) {
		return diagnostics, errResourceDefItemEmpty(
			path,
			provider.ResourceDefinitionsSchemaTypeObject,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	hasNilValue := node.Fields == nil
	if hasNilValue && validateAgainstSchema.Nullable {
		return diagnostics, nil
	}

	if hasNilValue {
		specType := deriveMappingNodeResourceDefinitionsType(node)

		return diagnostics, errResourceDefInvalidType(
			path,
			specType,
			provider.ResourceDefinitionsSchemaTypeObject,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	var errs []error

	for attrName, attrSchema := range validateAgainstSchema.Attributes {
		attrPath := fmt.Sprintf("%s.%s", path, attrName)
		attrNode, hasAttr := node.Fields[attrName]
		if !hasAttr {
			if slices.Contains(validateAgainstSchema.Required, attrName) {
				errs = append(errs, errResourceDefMissingRequiredField(
					attrPath,
					attrName,
					attrSchema.Type,
					selectMappingNodeLocation(node, parentLocation),
				))
			}
		} else {
			attrDiagnostics, err := validateResourceDefinition(
				ctx,
				resourceName,
				resourceType,
				resourceDerivedFromTemplate,
				attrNode,
				parentLocation,
				attrSchema,
				bpSchema,
				params,
				funcRegistry,
				refChainCollector,
				resourceRegistry,
				attrPath,
				depth+1,
			)
			diagnostics = append(diagnostics, attrDiagnostics...)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	for fieldName, fieldNode := range node.Fields {
		fieldPath := fmt.Sprintf("%s.%s", path, fieldName)
		if _, hasAttr := validateAgainstSchema.Attributes[fieldName]; !hasAttr {
			errs = append(errs, errResourceDefUnknownField(
				fieldPath,
				fieldName,
				selectMappingNodeLocation(fieldNode, parentLocation),
			))
		}
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func validateResourceDefinitionMap(
	ctx context.Context,
	resourceName string,
	resourceType string,
	resourceDerivedFromTemplate bool,
	node *core.MappingNode,
	parentLocation *source.Meta,
	validateAgainstSchema *provider.ResourceDefinitionsSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector refgraph.RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
	path string,
	depth int,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(node) {
		return diagnostics, errResourceDefItemEmpty(
			path,
			provider.ResourceDefinitionsSchemaTypeMap,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	hasNilValue := node.Fields == nil
	if hasNilValue && validateAgainstSchema.Nullable {
		return diagnostics, nil
	}

	if hasNilValue {
		specType := deriveMappingNodeResourceDefinitionsType(node)

		return diagnostics, errResourceDefInvalidType(
			path,
			specType,
			provider.ResourceDefinitionsSchemaTypeMap,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	var errs []error

	for fieldName, fieldNode := range node.Fields {
		fieldPath := fmt.Sprintf("%s.%s", path, fieldName)
		fieldDiagnostics, err := validateResourceDefinition(
			ctx,
			resourceName,
			resourceType,
			resourceDerivedFromTemplate,
			fieldNode,
			parentLocation,
			validateAgainstSchema.MapValues,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			fieldPath,
			depth+1,
		)
		diagnostics = append(diagnostics, fieldDiagnostics...)
		if err != nil {
			errs = append(errs, err)
		}

	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func validateResourceDefinitionArray(
	ctx context.Context,
	resourceName string,
	resourceType string,
	resourceDerivedFromTemplate bool,
	node *core.MappingNode,
	parentLocation *source.Meta,
	validateAgainstSchema *provider.ResourceDefinitionsSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector refgraph.RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
	path string,
	depth int,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(node) {
		return diagnostics, errResourceDefItemEmpty(
			path,
			provider.ResourceDefinitionsSchemaTypeArray,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	hasNilValue := node.Items == nil
	if hasNilValue && validateAgainstSchema.Nullable {
		return diagnostics, nil
	}

	if hasNilValue {
		specType := deriveMappingNodeResourceDefinitionsType(node)

		return diagnostics, errResourceDefInvalidType(
			path,
			specType,
			provider.ResourceDefinitionsSchemaTypeArray,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	var errs []error

	for itemIndex, itemNode := range node.Items {
		itemPath := fmt.Sprintf("%s[%d]", path, itemIndex)
		fieldDiagnostics, err := validateResourceDefinition(
			ctx,
			resourceName,
			resourceType,
			resourceDerivedFromTemplate,
			itemNode,
			parentLocation,
			validateAgainstSchema.Items,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			itemPath,
			depth+1,
		)
		diagnostics = append(diagnostics, fieldDiagnostics...)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func validateResourceDefinitionString(
	ctx context.Context,
	resourceName string,
	resourceDerivedFromTemplate bool,
	node *core.MappingNode,
	parentLocation *source.Meta,
	schema *provider.ResourceDefinitionsSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector refgraph.RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
	path string,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(node) {
		return diagnostics, errResourceDefItemEmpty(
			path,
			provider.ResourceDefinitionsSchemaTypeString,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	hasNilValue := (node.Scalar == nil ||
		(node.Scalar != nil && node.Scalar.StringValue == nil)) &&
		node.StringWithSubstitutions == nil

	if hasNilValue && schema.Nullable {
		return diagnostics, nil
	}

	if hasNilValue {
		specType := deriveMappingNodeResourceDefinitionsType(node)
		if specType == "" {
			return diagnostics, errResourceDefItemEmpty(
				path,
				provider.ResourceDefinitionsSchemaTypeString,
				selectMappingNodeLocation(node, parentLocation),
			)
		}
		return diagnostics, errResourceDefInvalidType(
			path,
			specType,
			provider.ResourceDefinitionsSchemaTypeString,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	if len(schema.AllowedValues) > 0 {
		allowedValueDiagnostics, err := validateResourceDefinitionAllowedValues(
			node,
			schema,
			path,
			selectMappingNodeLocation(node, parentLocation),
		)
		diagnostics = append(diagnostics, allowedValueDiagnostics...)
		if err != nil {
			return diagnostics, err
		}
	}

	if node.StringWithSubstitutions != nil {
		subDiagnostics, err := validateResourceDefinitionSubstitution(
			ctx,
			resourceName,
			resourceDerivedFromTemplate,
			node.StringWithSubstitutions,
			substitutions.ResolvedSubExprTypeString,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			path,
		)
		diagnostics = append(diagnostics, subDiagnostics...)
		if err != nil {
			return diagnostics, err
		}
	}

	return diagnostics, nil
}

func validateResourceDefinitionInteger(
	ctx context.Context,
	resourceName string,
	resourceDerivedFromTemplate bool,
	node *core.MappingNode,
	parentLocation *source.Meta,
	schema *provider.ResourceDefinitionsSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector refgraph.RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
	path string,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(node) {
		return diagnostics, errResourceDefItemEmpty(
			path,
			provider.ResourceDefinitionsSchemaTypeInteger,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	hasNilValue := (node.Scalar == nil ||
		(node.Scalar != nil && node.Scalar.IntValue == nil)) &&
		node.StringWithSubstitutions == nil

	if hasNilValue && schema.Nullable {
		return diagnostics, nil
	}

	if hasNilValue {
		specType := deriveMappingNodeResourceDefinitionsType(node)
		if specType == "" {
			return diagnostics, errResourceDefItemEmpty(
				path,
				provider.ResourceDefinitionsSchemaTypeInteger,
				selectMappingNodeLocation(node, parentLocation),
			)
		}

		return diagnostics, errResourceDefInvalidType(
			path,
			specType,
			provider.ResourceDefinitionsSchemaTypeInteger,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	if len(schema.AllowedValues) > 0 {
		allowedValueDiagnostics, err := validateResourceDefinitionAllowedValues(
			node,
			schema,
			path,
			selectMappingNodeLocation(node, parentLocation),
		)
		diagnostics = append(diagnostics, allowedValueDiagnostics...)
		if err != nil {
			return diagnostics, err
		}
	}

	if node.StringWithSubstitutions != nil {
		subDiagnostics, err := validateResourceDefinitionSubstitution(
			ctx,
			resourceName,
			resourceDerivedFromTemplate,
			node.StringWithSubstitutions,
			substitutions.ResolvedSubExprTypeInteger,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			path,
		)
		diagnostics = append(diagnostics, subDiagnostics...)
		if err != nil {
			return diagnostics, err
		}
	}

	return diagnostics, nil
}

func validateResourceDefinitionFloat(
	ctx context.Context,
	resourceName string,
	resourceDerivedFromTemplate bool,
	node *core.MappingNode,
	parentLocation *source.Meta,
	schema *provider.ResourceDefinitionsSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector refgraph.RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
	path string,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(node) {
		return diagnostics, errResourceDefItemEmpty(
			path,
			provider.ResourceDefinitionsSchemaTypeFloat,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	hasNilValue := (node.Scalar == nil ||
		(node.Scalar != nil && node.Scalar.FloatValue == nil)) &&
		node.StringWithSubstitutions == nil

	if hasNilValue && schema.Nullable {
		return diagnostics, nil
	}

	if hasNilValue {
		specType := deriveMappingNodeResourceDefinitionsType(node)
		if specType == "" {
			return diagnostics, errResourceDefItemEmpty(
				path,
				provider.ResourceDefinitionsSchemaTypeFloat,
				selectMappingNodeLocation(node, parentLocation),
			)
		}

		return diagnostics, errResourceDefInvalidType(
			path,
			specType,
			provider.ResourceDefinitionsSchemaTypeFloat,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	if len(schema.AllowedValues) > 0 {
		allowedValueDiagnostics, err := validateResourceDefinitionAllowedValues(
			node,
			schema,
			path,
			selectMappingNodeLocation(node, parentLocation),
		)
		diagnostics = append(diagnostics, allowedValueDiagnostics...)
		if err != nil {
			return diagnostics, err
		}
	}

	if node.StringWithSubstitutions != nil {
		subDiagnostics, err := validateResourceDefinitionSubstitution(
			ctx,
			resourceName,
			resourceDerivedFromTemplate,
			node.StringWithSubstitutions,
			substitutions.ResolvedSubExprTypeFloat,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			path,
		)
		diagnostics = append(diagnostics, subDiagnostics...)
		if err != nil {
			return diagnostics, err
		}
	}

	return diagnostics, nil
}

func validateResourceDefinitionBoolean(
	ctx context.Context,
	resourceName string,
	resourceDerivedFromTemplate bool,
	node *core.MappingNode,
	parentLocation *source.Meta,
	schema *provider.ResourceDefinitionsSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector refgraph.RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
	path string,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(node) {
		return diagnostics, errResourceDefItemEmpty(
			path,
			provider.ResourceDefinitionsSchemaTypeBoolean,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	hasNilValue := (node.Scalar == nil ||
		(node.Scalar != nil && node.Scalar.BoolValue == nil)) &&
		node.StringWithSubstitutions == nil

	if hasNilValue && schema.Nullable {
		return diagnostics, nil
	}

	if hasNilValue {
		specType := deriveMappingNodeResourceDefinitionsType(node)
		if specType == "" {
			return diagnostics, errResourceDefItemEmpty(
				path,
				provider.ResourceDefinitionsSchemaTypeBoolean,
				selectMappingNodeLocation(node, parentLocation),
			)
		}

		return diagnostics, errResourceDefInvalidType(
			path,
			specType,
			provider.ResourceDefinitionsSchemaTypeBoolean,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	if node.StringWithSubstitutions != nil {
		subDiagnostics, err := validateResourceDefinitionSubstitution(
			ctx,
			resourceName,
			resourceDerivedFromTemplate,
			node.StringWithSubstitutions,
			substitutions.ResolvedSubExprTypeBoolean,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			path,
		)
		diagnostics = append(diagnostics, subDiagnostics...)
		if err != nil {
			return diagnostics, err
		}
	}

	return diagnostics, nil
}

func validateResourceDefinitionUnion(
	ctx context.Context,
	resourceName string,
	resourceType string,
	resourceDerivedFromTemplate bool,
	spec *core.MappingNode,
	parentLocation *source.Meta,
	validateAgainstSchema *provider.ResourceDefinitionsSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector refgraph.RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
	path string,
	depth int,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(spec) {
		return diagnostics, errResourceDefUnionItemEmpty(
			path,
			validateAgainstSchema.OneOf,
			selectMappingNodeLocation(spec, parentLocation),
		)
	}

	foundMatch := false
	i := 0
	for !foundMatch && i < len(validateAgainstSchema.OneOf) {
		unionSchema := validateAgainstSchema.OneOf[i]
		unionDiagnostics, err := validateResourceDefinition(
			ctx,
			resourceName,
			resourceType,
			resourceDerivedFromTemplate,
			spec,
			parentLocation,
			unionSchema,
			bpSchema,
			params,
			funcRegistry,
			refChainCollector,
			resourceRegistry,
			path,
			depth,
		)
		diagnostics = append(diagnostics, unionDiagnostics...)
		if err == nil {
			foundMatch = true
		}
		i += 1
	}

	if !foundMatch {
		return diagnostics, errResourceDefUnionInvalidType(
			path,
			validateAgainstSchema.OneOf,
			selectMappingNodeLocation(spec, parentLocation),
		)
	}

	return diagnostics, nil
}

func validateResourceDefinitionSubstitution(
	ctx context.Context,
	resourceName string,
	resourceDerivedFromTemplate bool,
	value *substitutions.StringOrSubstitutions,
	expectedResolvedType substitutions.ResolvedSubExprType,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector refgraph.RefChainCollector,
	resourceRegistry resourcehelpers.Registry,
	path string,
) ([]*core.Diagnostic, error) {
	if value == nil {
		return []*core.Diagnostic{}, nil
	}

	resourceIdentifier := core.ResourceElementID(resourceName)
	errs := []error{}
	diagnostics := []*core.Diagnostic{}

	if len(value.Values) > 1 && expectedResolvedType != substitutions.ResolvedSubExprTypeString {
		return diagnostics, errInvalidResourceDefSubType(
			// StringOrSubstitutions with multiple values is an
			// interpolated string.
			string(substitutions.ResolvedSubExprTypeString),
			path,
			string(expectedResolvedType),
			value.SourceMeta,
		)
	}

	for _, stringOrSub := range value.Values {
		if stringOrSub.SubstitutionValue != nil {
			resolvedType, subDiagnostics, err := ValidateSubstitution(
				ctx,
				stringOrSub.SubstitutionValue,
				nil,
				bpSchema,
				resourceDerivedFromTemplate,
				resourceIdentifier,
				path,
				params,
				funcRegistry,
				refChainCollector,
				resourceRegistry,
			)
			if err != nil {
				errs = append(errs, err)
			} else {
				diagnostics = append(diagnostics, subDiagnostics...)
				if resolvedType != string(expectedResolvedType) &&
					resolvedType != string(substitutions.ResolvedSubExprTypeAny) {
					errs = append(errs, errInvalidResourceDefSubType(
						resolvedType,
						path,
						string(expectedResolvedType),
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

// A maximum number of allowed values to show in error and warning messages.
const maxShowAllowedValues = 5

func validateResourceDefinitionAllowedValues(
	node *core.MappingNode,
	schema *provider.ResourceDefinitionsSchema,
	path string,
	location *source.Meta,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	allowedValuesText := createAllowedValuesText(schema.AllowedValues, maxShowAllowedValues)
	if !core.IsScalarMappingNode(node) && node.StringWithSubstitutions != nil {
		if schema.Type != provider.ResourceDefinitionsSchemaTypeString &&
			// Interpolated strings will be resolved as strings,
			// an interpolated string is one that contains a combination of
			// strings and substitutions or has more than one substitution.
			isInterpolatedString(node.StringWithSubstitutions) {
			return diagnostics, errResourceDefInvalidType(
				path,
				deriveMappingNodeResourceDefinitionsType(node),
				schema.Type,
				selectMappingNodeLocation(node, location),
			)
		}

		// When a value is a string with substitutions and the field schema is a string,
		// we can not validate a value that is not yet resolved.
		// Warnings are useful to make practitioners aware of the possibility
		// of a failure during change staging or deployment for a field
		// that must be one of a fixed set of values.
		diagnostics = append(
			diagnostics,
			&core.Diagnostic{
				Level: core.DiagnosticLevelWarning,
				Message: fmt.Sprintf(
					"The value of %q contains substitutions and can not be validated against the allowed values. "+
						"When substitutions are resolved, this value must match one of the allowed values: %s",
					path,
					allowedValuesText,
				),
				Range: toDiagnosticRange(location, nil),
			},
		)
		return diagnostics, nil
	}

	inAllowedList := slices.ContainsFunc(
		schema.AllowedValues,
		func(allowedValue *core.MappingNode) bool {
			return core.IsScalarMappingNode(node) &&
				core.IsScalarMappingNode(allowedValue) &&
				node.Scalar.Equal(allowedValue.Scalar)
		},
	)

	if !inAllowedList {
		return diagnostics, errResourceDefNotAllowedValue(
			path,
			allowedValuesText,
			selectMappingNodeLocation(node, location),
		)
	}

	return diagnostics, nil
}

func createAllowedValuesText(allowedValues []*core.MappingNode, maxCount int) string {
	if len(allowedValues) <= maxCount {
		return mappingNodesToCommaSeparatedString(allowedValues)
	}

	// Show only the first `maxCount` allowed values.
	allowedValuesStr := mappingNodesToCommaSeparatedString(allowedValues[:maxCount])
	return fmt.Sprintf("%s, and %d more",
		allowedValuesStr,
		len(allowedValues)-maxCount,
	)
}

func mappingNodesToCommaSeparatedString(nodes []*core.MappingNode) string {
	values := make([]string, len(nodes))
	for i, node := range nodes {
		if core.IsScalarMappingNode(node) {
			values[i] = node.Scalar.ToString()
		} else {
			values[i] = "<unknown>"
		}
	}
	return strings.Join(values, ", ")
}

func isInterpolatedString(value *substitutions.StringOrSubstitutions) bool {
	return !substitutions.IsNilStringSubs(value) &&
		(len(value.Values) > 1 || value.Values[0].StringValue != nil)
}

func selectMappingNodeLocation(node *core.MappingNode, parentLocation *source.Meta) *source.Meta {
	if node != nil && node.SourceMeta != nil {
		return node.SourceMeta
	}

	return parentLocation
}

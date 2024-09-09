package validation

import (
	"context"
	"fmt"
	"slices"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
)

// ValidateResourceSpec validates the `spec` field of a resource.
// In a blueprint, this is a free form object that can hold complex structures
// that define a resource's configuration.
// Each resource has its own schema that is defined by the provider,
// this function validates against that schema and runs custom validation
// defined by the provider.
// This will only traverse up to `MappingNodeMaxTraverseDepth` levels deep,
// if the depth is exceeded, validation will not be performed on further elements.
func ValidateResourceSpec(
	ctx context.Context,
	name string,
	resourceType string,
	resource *schema.Resource,
	resourceLocation *source.Meta,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	specDefinition, err := loadResourceSpecDefinition(
		ctx,
		resource.Type,
		name,
		resource.SourceMeta,
		params,
		resourceRegistry,
	)
	if err != nil {
		// If there is no spec definition for the resource type,
		// we can't validate the spec.
		return diagnostics, err
	}

	var errs []error
	path := fmt.Sprintf("resources.%s.spec", name)
	specDiagnostics, err := validateResourceSpec(
		ctx,
		name,
		resourceType,
		resource.Spec,
		resourceLocation,
		specDefinition.Schema,
		bpSchema,
		params,
		funcRegistry,
		refChainCollector,
		resourceRegistry,
		path,
		/* depth */ 0,
	)
	diagnostics = append(diagnostics, specDiagnostics...)
	if err != nil {
		errs = append(errs, err)
	}

	customOutput, err := resourceRegistry.CustomValidate(
		ctx,
		resourceType,
		&provider.ResourceValidateInput{
			SchemaResource: resource,
			Params:         params,
		},
	)
	if customOutput != nil {
		diagnostics = append(diagnostics, customOutput.Diagnostics...)
	}
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func loadResourceSpecDefinition(
	ctx context.Context,
	resourceType string,
	resourceName string,
	location *source.Meta,
	params core.BlueprintParams,
	resourceRegistry provider.ResourceRegistry,
) (*provider.ResourceSpecDefinition, error) {
	specDefOutput, err := resourceRegistry.GetSpecDefinition(
		ctx,
		resourceType,
		&provider.ResourceGetSpecDefinitionInput{
			Params: params,
		},
	)
	if err != nil {
		return nil, err
	}

	if specDefOutput.SpecDefinition == nil {
		return nil, errResourceTypeMissingSpecDefinition(
			resourceName,
			resourceType,
			/* inSubstitution */ false,
			location,
		)
	}

	if specDefOutput.SpecDefinition.Schema == nil {
		return nil, errResourceTypeSpecDefMissingSchema(
			resourceName,
			resourceType,
			/* inSubstitution */ false,
			location,
		)
	}

	return specDefOutput.SpecDefinition, nil
}

func validateResourceSpec(
	ctx context.Context,
	resourceName string,
	resourceType string,
	spec *core.MappingNode,
	parentLocation *source.Meta,
	validateAgainstSchema *provider.ResourceSpecSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
	path string,
	depth int,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}
	if depth > MappingNodeMaxTraverseDepth {
		return diagnostics, nil
	}

	isEmpty := isMappingNodeEmpty(spec)
	if isEmpty && validateAgainstSchema.Nullable {
		return diagnostics, nil
	}

	switch validateAgainstSchema.Type {
	case provider.ResourceSpecSchemaTypeObject:
		return validateResourceSpecObject(
			ctx,
			resourceName,
			resourceType,
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
	case provider.ResourceSpecSchemaTypeMap:
		return validateResourceSpecMap(
			ctx,
			resourceName,
			resourceType,
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
	case provider.ResourceSpecSchemaTypeArray:
		return validateResourceSpecArray(
			ctx,
			resourceName,
			resourceType,
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
	case provider.ResourceSpecSchemaTypeString:
		return validateResourceSpecString(
			ctx,
			resourceName,
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
	case provider.ResourceSpecSchemaTypeInteger:
		return validateResourceSpecInteger(
			ctx,
			resourceName,
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
	case provider.ResourceSpecSchemaTypeFloat:
		return validateResourceSpecFloat(
			ctx,
			resourceName,
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
	case provider.ResourceSpecSchemaTypeBoolean:
		return validateResourceSpecBoolean(
			ctx,
			resourceName,
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
	case provider.ResourceSpecSchemaTypeUnion:
		return validateResourceSpecUnion(
			ctx,
			resourceName,
			resourceType,
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
		return diagnostics, provider.ErrUnknownResourceSpecType(
			validateAgainstSchema.Type,
			resourceType,
		)
	}
}

func validateResourceSpecObject(
	ctx context.Context,
	resourceName string,
	resourceType string,
	node *core.MappingNode,
	parentLocation *source.Meta,
	validateAgainstSchema *provider.ResourceSpecSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
	path string,
	depth int,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(node) {
		return diagnostics, errResourceSpecItemEmpty(
			path,
			provider.ResourceSpecSchemaTypeObject,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	hasNilValue := node.Fields == nil
	if hasNilValue && validateAgainstSchema.Nullable {
		return diagnostics, nil
	}

	if hasNilValue {
		specType := deriveMappingNodeResourceSpecType(node)

		return diagnostics, errResourceSpecInvalidType(
			path,
			specType,
			provider.ResourceSpecSchemaTypeObject,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	var errs []error

	for attrName, attrSchema := range validateAgainstSchema.Attributes {
		attrPath := fmt.Sprintf("%s.%s", path, attrName)
		attrNode, hasAttr := node.Fields[attrName]
		if !hasAttr {
			if slices.Contains(validateAgainstSchema.Required, attrName) {
				errs = append(errs, errResourceSpecMissingRequiredField(
					attrPath,
					attrName,
					attrSchema.Type,
					selectMappingNodeLocation(node, parentLocation),
				))
			}
		} else {
			attrDiagnostics, err := validateResourceSpec(
				ctx,
				resourceName,
				resourceType,
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

	if len(errs) > 0 {
		return diagnostics, ErrMultipleValidationErrors(errs)
	}

	return diagnostics, nil
}

func validateResourceSpecMap(
	ctx context.Context,
	resourceName string,
	resourceType string,
	node *core.MappingNode,
	parentLocation *source.Meta,
	validateAgainstSchema *provider.ResourceSpecSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
	path string,
	depth int,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(node) {
		return diagnostics, errResourceSpecItemEmpty(
			path,
			provider.ResourceSpecSchemaTypeMap,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	hasNilValue := node.Fields == nil
	if hasNilValue && validateAgainstSchema.Nullable {
		return diagnostics, nil
	}

	if hasNilValue {
		specType := deriveMappingNodeResourceSpecType(node)

		return diagnostics, errResourceSpecInvalidType(
			path,
			specType,
			provider.ResourceSpecSchemaTypeMap,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	var errs []error

	for fieldName, fieldNode := range node.Fields {
		fieldPath := fmt.Sprintf("%s.%s", path, fieldName)
		fieldDiagnostics, err := validateResourceSpec(
			ctx,
			resourceName,
			resourceType,
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

func validateResourceSpecArray(
	ctx context.Context,
	resourceName string,
	resourceType string,
	node *core.MappingNode,
	parentLocation *source.Meta,
	validateAgainstSchema *provider.ResourceSpecSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
	path string,
	depth int,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(node) {
		return diagnostics, errResourceSpecItemEmpty(
			path,
			provider.ResourceSpecSchemaTypeArray,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	hasNilValue := node.Items == nil
	if hasNilValue && validateAgainstSchema.Nullable {
		return diagnostics, nil
	}

	if hasNilValue {
		specType := deriveMappingNodeResourceSpecType(node)

		return diagnostics, errResourceSpecInvalidType(
			path,
			specType,
			provider.ResourceSpecSchemaTypeArray,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	var errs []error

	for itemIndex, itemNode := range node.Items {
		itemPath := fmt.Sprintf("%s[%d]", path, itemIndex)
		fieldDiagnostics, err := validateResourceSpec(
			ctx,
			resourceName,
			resourceType,
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

func validateResourceSpecString(
	ctx context.Context,
	resourceName string,
	node *core.MappingNode,
	parentLocation *source.Meta,
	schema *provider.ResourceSpecSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
	path string,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(node) {
		return diagnostics, errResourceSpecItemEmpty(
			path,
			provider.ResourceSpecSchemaTypeString,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	hasNilValue := (node.Literal == nil ||
		(node.Literal != nil && node.Literal.StringValue == nil)) &&
		node.StringWithSubstitutions == nil

	if hasNilValue && schema.Nullable {
		return diagnostics, nil
	}

	if hasNilValue {
		specType := deriveMappingNodeResourceSpecType(node)
		if specType == "" {
			return diagnostics, errResourceSpecItemEmpty(
				path,
				provider.ResourceSpecSchemaTypeString,
				selectMappingNodeLocation(node, parentLocation),
			)
		}
		return diagnostics, errResourceSpecInvalidType(
			path,
			specType,
			provider.ResourceSpecSchemaTypeString,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	if node.StringWithSubstitutions != nil {
		subDiagnostics, err := validateResourceSpecSubstitution(
			ctx,
			resourceName,
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

func validateResourceSpecInteger(
	ctx context.Context,
	resourceName string,
	node *core.MappingNode,
	parentLocation *source.Meta,
	schema *provider.ResourceSpecSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
	path string,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(node) {
		return diagnostics, errResourceSpecItemEmpty(
			path,
			provider.ResourceSpecSchemaTypeInteger,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	hasNilValue := (node.Literal == nil ||
		(node.Literal != nil && node.Literal.IntValue == nil)) &&
		node.StringWithSubstitutions == nil

	if hasNilValue && schema.Nullable {
		return diagnostics, nil
	}

	if hasNilValue {
		specType := deriveMappingNodeResourceSpecType(node)
		if specType == "" {
			return diagnostics, errResourceSpecItemEmpty(
				path,
				provider.ResourceSpecSchemaTypeInteger,
				selectMappingNodeLocation(node, parentLocation),
			)
		}

		return diagnostics, errResourceSpecInvalidType(
			path,
			specType,
			provider.ResourceSpecSchemaTypeInteger,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	if node.StringWithSubstitutions != nil {
		subDiagnostics, err := validateResourceSpecSubstitution(
			ctx,
			resourceName,
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

func validateResourceSpecFloat(
	ctx context.Context,
	resourceName string,
	node *core.MappingNode,
	parentLocation *source.Meta,
	schema *provider.ResourceSpecSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
	path string,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(node) {
		return diagnostics, errResourceSpecItemEmpty(
			path,
			provider.ResourceSpecSchemaTypeFloat,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	hasNilValue := (node.Literal == nil ||
		(node.Literal != nil && node.Literal.FloatValue == nil)) &&
		node.StringWithSubstitutions == nil

	if hasNilValue && schema.Nullable {
		return diagnostics, nil
	}

	if hasNilValue {
		specType := deriveMappingNodeResourceSpecType(node)
		if specType == "" {
			return diagnostics, errResourceSpecItemEmpty(
				path,
				provider.ResourceSpecSchemaTypeFloat,
				selectMappingNodeLocation(node, parentLocation),
			)
		}

		return diagnostics, errResourceSpecInvalidType(
			path,
			specType,
			provider.ResourceSpecSchemaTypeFloat,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	if node.StringWithSubstitutions != nil {
		subDiagnostics, err := validateResourceSpecSubstitution(
			ctx,
			resourceName,
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

func validateResourceSpecBoolean(
	ctx context.Context,
	resourceName string,
	node *core.MappingNode,
	parentLocation *source.Meta,
	schema *provider.ResourceSpecSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
	path string,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(node) {
		return diagnostics, errResourceSpecItemEmpty(
			path,
			provider.ResourceSpecSchemaTypeBoolean,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	hasNilValue := (node.Literal == nil ||
		(node.Literal != nil && node.Literal.BoolValue == nil)) &&
		node.StringWithSubstitutions == nil

	if hasNilValue && schema.Nullable {
		return diagnostics, nil
	}

	if hasNilValue {
		specType := deriveMappingNodeResourceSpecType(node)
		if specType == "" {
			return diagnostics, errResourceSpecItemEmpty(
				path,
				provider.ResourceSpecSchemaTypeBoolean,
				selectMappingNodeLocation(node, parentLocation),
			)
		}

		return diagnostics, errResourceSpecInvalidType(
			path,
			specType,
			provider.ResourceSpecSchemaTypeBoolean,
			selectMappingNodeLocation(node, parentLocation),
		)
	}

	if node.StringWithSubstitutions != nil {
		subDiagnostics, err := validateResourceSpecSubstitution(
			ctx,
			resourceName,
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

func validateResourceSpecUnion(
	ctx context.Context,
	resourceName string,
	resourceType string,
	spec *core.MappingNode,
	parentLocation *source.Meta,
	validateAgainstSchema *provider.ResourceSpecSchema,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
	path string,
	depth int,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}

	if isMappingNodeEmpty(spec) {
		return diagnostics, errResourceSpecUnionItemEmpty(
			path,
			validateAgainstSchema.OneOf,
			selectMappingNodeLocation(spec, parentLocation),
		)
	}

	foundMatch := false
	i := 0
	for !foundMatch && i < len(validateAgainstSchema.OneOf) {
		unionSchema := validateAgainstSchema.OneOf[i]
		unionDiagnostics, err := validateResourceSpec(
			ctx,
			resourceName,
			resourceType,
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
		return diagnostics, errResourceSpecUnionInvalidType(
			path,
			validateAgainstSchema.OneOf,
			selectMappingNodeLocation(spec, parentLocation),
		)
	}

	return diagnostics, nil
}

func validateResourceSpecSubstitution(
	ctx context.Context,
	resourceName string,
	value *substitutions.StringOrSubstitutions,
	expectedResolvedType substitutions.ResolvedSubExprType,
	bpSchema *schema.Blueprint,
	params core.BlueprintParams,
	funcRegistry provider.FunctionRegistry,
	refChainCollector RefChainCollector,
	resourceRegistry provider.ResourceRegistry,
	path string,
) ([]*core.Diagnostic, error) {
	if value == nil {
		return []*core.Diagnostic{}, nil
	}

	resourceIdentifier := fmt.Sprintf("resources.%s", resourceName)
	errs := []error{}
	diagnostics := []*core.Diagnostic{}

	if len(value.Values) > 1 && expectedResolvedType != substitutions.ResolvedSubExprTypeString {
		return diagnostics, errInvalidResourceSpecSubType(
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
				resourceIdentifier,
				params,
				funcRegistry,
				refChainCollector,
				resourceRegistry,
			)
			if err != nil {
				errs = append(errs, err)
			} else {
				diagnostics = append(diagnostics, subDiagnostics...)
				if resolvedType != string(expectedResolvedType) {
					errs = append(errs, errInvalidResourceSpecSubType(
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

func selectMappingNodeLocation(node *core.MappingNode, parentLocation *source.Meta) *source.Meta {
	if node != nil && node.SourceMeta != nil {
		return node.SourceMeta
	}

	return parentLocation
}

package validation

import (
	"context"
	"fmt"
	"math"
	"strings"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"github.com/two-hundred/celerity/libs/common/core"
)

// ValidateSubstitution validates a substitution usage in a blueprint.
//
// usedIn is the path to the element in the blueprint where the substitution is used.
// This should be in the format of "{elementType}.{elementName}"
// For example, "values.myValue" or "resources.myResource"
//
// funcRegistry provides a registry of functions that can be used in the substitution.
//
// This returns a string containing the type of the resolved value for the substitution
// where it can be determined, an empty string otherwise.
// The caller is responsible for ensuring that the resolved value type is compatible with
// the context where the substitution is used.
// It also returns a list of diagnostics that were generated during the
// validation process and an error if the validation process failed.
func ValidateSubstitution(
	ctx context.Context,
	sub *substitutions.Substitution,
	nextLocation *source.Meta,
	bpSchema *schema.Blueprint,
	usedIn string,
	funcRegistry provider.FunctionRegistry,
	refChainCollector *RefChainCollector,
) (string, []*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if sub == nil {
		return "", diagnostics, nil
	}

	if sub.Function != nil {
		return validateFunctionSubstitution(
			ctx,
			sub.Function,
			nextLocation,
			bpSchema,
			usedIn,
			funcRegistry,
			refChainCollector,
		)
	}

	if sub.BoolValue != nil {
		return string(substitutions.ResolvedSubExprTypeBoolean), diagnostics, nil
	}

	if sub.StringValue != nil {
		return string(substitutions.ResolvedSubExprTypeString), diagnostics, nil
	}

	if sub.IntValue != nil {
		return string(substitutions.ResolvedSubExprTypeInteger), diagnostics, nil
	}

	if sub.FloatValue != nil {
		return string(substitutions.ResolvedSubExprTypeFloat), diagnostics, nil
	}

	if sub.Variable != nil {
		return validateVariableSubstitution(sub.Variable, bpSchema)
	}

	if sub.ValueReference != nil {
		return validateValueSubstitution(sub.ValueReference, bpSchema, usedIn, refChainCollector)
	}

	if sub.ElemReference != nil {
		return validateElemReferenceSubstitution(
			"element",
			sub.ElemReference.SourceMeta,
			bpSchema,
			usedIn,
		)
	}

	if sub.ElemIndexReference != nil {
		return validateElemReferenceSubstitution(
			"index",
			sub.ElemIndexReference.SourceMeta,
			bpSchema,
			usedIn,
		)
	}

	if sub.ResourceProperty != nil {
		return validateResourcePropertySubstitution(
			sub.ResourceProperty,
			bpSchema,
			usedIn,
			refChainCollector,
		)
	}

	if sub.DataSourceProperty != nil {
		return validateDataSourcePropertySubstitution(
			sub.DataSourceProperty,
			bpSchema,
			usedIn,
			refChainCollector,
		)
	}

	if sub.Child != nil {
		return validateChildSubstitution(sub.Child, bpSchema, usedIn, refChainCollector)
	}

	return "", diagnostics, nil
}

func validateVariableSubstitution(
	subVar *substitutions.SubstitutionVariable,
	bpSchema *schema.Blueprint,
) (string, []*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	varName := subVar.VariableName
	varSchema, hasVar := bpSchema.Variables.Values[varName]
	if !hasVar {
		return "", diagnostics, errSubVarNotFound(varName, subVar.SourceMeta)
	}

	// Variable references aren't collected with the reference cycle service
	// as cycles involving variable references are not possible;
	// this is because references are not
	// supported in variable definitions.

	return subVarType(varSchema.Type), diagnostics, nil
}

func validateValueSubstitution(
	subVal *substitutions.SubstitutionValueReference,
	bpSchema *schema.Blueprint,
	usedIn string,
	refChainCollector *RefChainCollector,
) (string, []*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	valName := subVal.ValueName
	valSchema, hasVal := bpSchema.Values.Values[valName]
	if !hasVal {
		return "", diagnostics, errSubValNotFound(valName, subVal.SourceMeta)
	}

	if usedIn == fmt.Sprintf("values.%s", valName) {
		return "", diagnostics, errSubValSelfReference(valName, subVal.SourceMeta)
	}

	refChainCollector.Collect(usedIn, fmt.Sprintf("values.%s", valName), valSchema)

	return subValType(valSchema.Type.Value), diagnostics, nil
}

func validateElemReferenceSubstitution(
	elemRefType string,
	location *source.Meta,
	bpSchema *schema.Blueprint,
	usedIn string,
) (string, []*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}

	if !strings.HasPrefix(usedIn, "resources.") {
		return "", diagnostics, errSubElemRefNotInResource(elemRefType, location)
	}

	resourceName := usedIn[10:]
	resource, hasResource := bpSchema.Resources.Values[resourceName]
	if !hasResource {
		return "", diagnostics, errSubElemRefResourceNotFound(elemRefType, resourceName, location)
	}

	if resource.Each == nil {
		return "", diagnostics, errSubElemRefResourceNotEach(elemRefType, resourceName, location)
	}

	// Element (and element index) references aren't collected with the reference cycle service
	// as cycles involving element references are not possible;
	// this is because elements are a reference to a local value that is only
	// scoped to the resource where the `each` property is defined.

	// The type of element references isn't known until runtime
	// as it dependent on the `each` property of the resource.
	resolvedType := string(substitutions.ResolvedSubExprTypeAny)
	if elemRefType == "index" {
		resolvedType = string(substitutions.ResolvedSubExprTypeInteger)
	}
	return resolvedType, diagnostics, nil
}

func validateResourcePropertySubstitution(
	subResourceProp *substitutions.SubstitutionResourceProperty,
	bpSchema *schema.Blueprint,
	usedIn string,
	refChainCollector *RefChainCollector,
) (string, []*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	resourceName := subResourceProp.ResourceName
	resourceSchema, hasResource := bpSchema.Resources.Values[resourceName]
	if !hasResource {
		return "", diagnostics, errSubResourceNotFound(resourceName, subResourceProp.SourceMeta)
	}

	if usedIn == fmt.Sprintf("resources.%s", resourceName) {
		return "", diagnostics, errSubResourceSelfReference(resourceName, subResourceProp.SourceMeta)
	}

	// TODO: Check `.spec.*` and `.metadata.*` paths against the resource spec.
	// Resolve the type of the property being referenced if under spec or metadata.

	refChainCollector.Collect(usedIn, fmt.Sprintf("resources.%s", resourceName), resourceSchema)

	return string(substitutions.ResolvedSubExprTypeAny), diagnostics, nil
}

func validateDataSourcePropertySubstitution(
	subDataSourceProp *substitutions.SubstitutionDataSourceProperty,
	bpSchema *schema.Blueprint,
	usedIn string,
	refChainCollector *RefChainCollector,
) (string, []*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	dataSourceName := subDataSourceProp.DataSourceName
	dataSourceSchema, hasDataSource := bpSchema.DataSources.Values[dataSourceName]
	if !hasDataSource {
		return "", diagnostics, errSubDataSourceNotFound(dataSourceName, subDataSourceProp.SourceMeta)
	}

	if usedIn == fmt.Sprintf("datasources.%s", dataSourceName) {
		return "", diagnostics, errSubDataSourceSelfReference(dataSourceName, subDataSourceProp.SourceMeta)
	}

	// TODO: Check `.*` paths against the data source exported fields.
	// Resolve the type of the property being referenced.

	refChainCollector.Collect(usedIn, fmt.Sprintf("datasources.%s", dataSourceName), dataSourceSchema)

	return string(substitutions.ResolvedSubExprTypeAny), diagnostics, nil
}

func validateChildSubstitution(
	subChild *substitutions.SubstitutionChild,
	bpSchema *schema.Blueprint,
	usedIn string,
	refChainCollector *RefChainCollector,
) (string, []*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	childName := subChild.ChildName
	childSchema, hasChild := bpSchema.Include.Values[childName]
	if !hasChild {
		return "", diagnostics, errSubChildBlueprintNotFound(childName, subChild.SourceMeta)
	}

	if usedIn == fmt.Sprintf("children.%s", childName) {
		return "", diagnostics, errSubChildBlueprintSelfReference(childName, subChild.SourceMeta)
	}

	refChainCollector.Collect(usedIn, fmt.Sprintf("children.%s", childName), childSchema)

	// There is no way of knowing the exact type of the child blueprint exports
	// until runtime, so we return any to account for all possible types.
	return string(substitutions.ResolvedSubExprTypeAny), diagnostics, nil
}

func validateFunctionSubstitution(
	ctx context.Context,
	subFunc *substitutions.SubstitutionFunctionExpr,
	nextLocation *source.Meta,
	bpSchema *schema.Blueprint,
	usedIn string,
	funcRegistry provider.FunctionRegistry,
	refChainCollector *RefChainCollector,
) (string, []*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	funcName := string(subFunc.FunctionName)

	isCoreFunc := core.SliceContainsComparable(
		substitutions.CoreSubstitutionFunctions,
		subFunc.FunctionName,
	)

	hasFunc, err := funcRegistry.HasFunction(ctx, funcName)
	if err != nil {
		return "", diagnostics, err
	}

	if !hasFunc && !isCoreFunc {
		diagnostics = append(
			diagnostics,
			&bpcore.Diagnostic{
				Level: bpcore.DiagnosticLevelWarning,
				Message: fmt.Sprintf(
					"Function %q is not a core function, when staging changes and deploying,"+
						" you will need to make sure the provider is loaded.",
					funcName,
				),
				Range: toDiagnosticRange(subFunc.SourceMeta, nextLocation),
			},
		)
	}

	defOutput, err := funcRegistry.GetDefinition(
		ctx,
		funcName,
		&provider.FunctionGetDefinitionInput{},
	)
	if err != nil {
		return "", diagnostics, err
	}

	if len(subFunc.Arguments) != len(defOutput.Definition.Parameters) &&
		!subFuncTakesVariadicArgs(defOutput.Definition) {
		return "", diagnostics, errSubFuncInvalidNumberOfArgs(
			len(defOutput.Definition.Parameters),
			len(subFunc.Arguments),
			subFunc,
		)
	}

	var errs []error
	for i, arg := range subFunc.Arguments {
		nextLocation := (*source.Meta)(nil)
		if i+1 < len(subFunc.Arguments) {
			nextLocation = subFunc.Arguments[i+1].SourceMeta
		}

		resolveType, argDiagnostics, err := validateSubFuncArgument(
			ctx,
			arg,
			nextLocation,
			bpSchema,
			usedIn,
			funcName,
			funcRegistry,
			refChainCollector,
		)
		diagnostics = append(diagnostics, argDiagnostics...)
		if err != nil {
			errs = append(errs, err)
		}

		if err == nil {
			err = checkSubFuncArgType(defOutput.Definition, i, resolveType, funcName, arg.SourceMeta)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return "", diagnostics, ErrMultipleValidationErrors(errs)
	}

	return subFunctionReturnType(defOutput), diagnostics, nil
}

func checkSubFuncArgType(
	definition *function.Definition,
	argIndex int,
	resolveType string,
	funcName string,
	location *source.Meta,
) error {
	paramIndex := int(math.Min(float64(argIndex), float64(len(definition.Parameters)-1)))
	param := definition.Parameters[paramIndex]

	anyParam, isAny := param.(*function.AnyParameter)
	if isAny {
		err := validateSubFuncArgAnyType(anyParam.UnionTypes, resolveType, argIndex, funcName, location)
		if err != nil {
			return err
		}
		return nil
	}

	variadicParam, isVariadic := param.(*function.VariadicParameter)
	if isVariadic {
		anyTypeDef, isAny := variadicParam.Type.(*function.ValueTypeDefinitionAny)
		if isAny {
			err := validateSubFuncArgAnyType(anyTypeDef.UnionTypes, resolveType, argIndex, funcName, location)
			if err != nil {
				return err
			}
			return nil
		}

		if string(variadicParam.GetType()) != resolveType {
			return errSubFuncArgTypeMismatch(
				argIndex,
				string(variadicParam.GetType()),
				resolveType,
				funcName,
				location,
			)
		}
		return nil
	}

	if string(param.GetType()) != resolveType {
		return errSubFuncArgTypeMismatch(
			argIndex,
			string(param.GetType()),
			resolveType,
			funcName,
			location,
		)
	}

	return nil
}

func validateSubFuncArgAnyType(
	unionTypes []function.ValueTypeDefinition,
	resolveType string,
	argIndex int,
	funcName string,
	location *source.Meta,
) error {
	if len(unionTypes) == 0 {
		// Any type without a strict union type should allow all possible types.
		return nil
	}

	matchesUnionType := true
	i := 0
	for matchesUnionType && i < len(unionTypes) {
		if string(unionTypes[i].GetType()) != resolveType {
			matchesUnionType = false
		}
		i += 1
	}

	if !matchesUnionType {
		return errSubFuncArgTypeMismatch(
			argIndex,
			string(subFuncUnionTypeToString(unionTypes)),
			resolveType,
			funcName,
			location,
		)
	}
	return nil
}

func validateSubFuncArgument(
	ctx context.Context,
	arg *substitutions.SubstitutionFunctionArg,
	nextLocation *source.Meta,
	bpSchema *schema.Blueprint,
	usedIn string,
	funcName string,
	funcRegistry provider.FunctionRegistry,
	refChainCollector *RefChainCollector,
) (string, []*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	if arg == nil {
		return "", diagnostics, nil
	}

	if arg.Value == nil {
		return "", diagnostics, nil
	}

	if arg.Name != "" && funcName != string(substitutions.SubstitutionFunctionObject) {
		return "", diagnostics, errSubFuncNamedArgsNotAllowed(arg.Name, funcName, arg.SourceMeta)
	}

	return ValidateSubstitution(
		ctx,
		arg.Value,
		nextLocation,
		bpSchema,
		usedIn,
		funcRegistry,
		refChainCollector,
	)
}

func subFuncTakesVariadicArgs(def *function.Definition) bool {
	if len(def.Parameters) == 0 {
		return false
	}

	lastParam := def.Parameters[len(def.Parameters)-1]
	_, isVariadic := lastParam.(*function.VariadicParameter)
	return isVariadic
}

func subFunctionReturnType(defOutput *provider.FunctionGetDefinitionOutput) string {
	return subFunctionValueType(defOutput.Definition.Return.GetType())
}

func subFunctionValueType(valueType function.ValueType) string {
	switch valueType {
	case function.ValueTypeString:
		return string(substitutions.ResolvedSubExprTypeString)
	case function.ValueTypeInt32:
		return string(substitutions.ResolvedSubExprTypeInteger)
	case function.ValueTypeInt64:
		return string(substitutions.ResolvedSubExprTypeInteger)
	case function.ValueTypeFloat32:
		return string(substitutions.ResolvedSubExprTypeFloat)
	case function.ValueTypeFloat64:
		return string(substitutions.ResolvedSubExprTypeFloat)
	case function.ValueTypeBool:
		return string(substitutions.ResolvedSubExprTypeBoolean)
	case function.ValueTypeList:
		return string(substitutions.ResolvedSubExprTypeArray)
	case function.ValueTypeMap:
		return string(substitutions.ResolvedSubExprTypeObject)
	case function.ValueTypeObject:
		return string(substitutions.ResolvedSubExprTypeObject)
	case function.ValueTypeFunction:
		return string(substitutions.ResolvedSubExprTypeFunction)
	default:
		return string(substitutions.ResolvedSubExprTypeAny)
	}
}

func subVarType(varType schema.VariableType) string {
	switch varType {
	case schema.VariableTypeInteger:
		return string(substitutions.ResolvedSubExprTypeInteger)
	case schema.VariableTypeFloat:
		return string(substitutions.ResolvedSubExprTypeFloat)
	case schema.VariableTypeBoolean:
		return string(substitutions.ResolvedSubExprTypeBoolean)
	default:
		// Strings and custom variable types are treated as strings.
		return string(substitutions.ResolvedSubExprTypeString)
	}
}

func subValType(valType schema.ValueType) string {
	switch valType {
	case schema.ValueTypeInteger:
		return string(substitutions.ResolvedSubExprTypeInteger)
	case schema.ValueTypeFloat:
		return string(substitutions.ResolvedSubExprTypeFloat)
	case schema.ValueTypeBoolean:
		return string(substitutions.ResolvedSubExprTypeBoolean)
	case schema.ValueTypeArray:
		return string(substitutions.ResolvedSubExprTypeArray)
	case schema.ValueTypeObject:
		return string(substitutions.ResolvedSubExprTypeObject)
	default:
		return string(substitutions.ResolvedSubExprTypeString)
	}
}

func subFuncUnionTypeToString(unionTypes []function.ValueTypeDefinition) string {
	var sb strings.Builder
	sb.WriteString("(")
	for i, t := range unionTypes {
		sb.WriteString(string(t.GetType()))
		if i < len(unionTypes)-1 {
			sb.WriteString(" | ")
		}
	}
	sb.WriteString(")")
	return sb.String()
}

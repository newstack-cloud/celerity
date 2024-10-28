package substitutions

import (
	"fmt"
	"strings"
)

// SubstitutionsToString converts a representation of a string as a sequence
// of string literals and interpolated substitutions to a string.
//
// An example output of this function would be:
//
//	"GetOrderFunction-${variables.env}-${variables.version}"
func SubstitutionsToString(substitutionContext string, substitutions *StringOrSubstitutions) (string, error) {
	var b strings.Builder
	// Validation of substitutions as per the spec is done at the time of serialisation/deserialisation
	// primarily to be as efficient as possible.
	subErrors := []error{}
	for _, value := range substitutions.Values {
		err := writeStringOrSubstitution(substitutionContext, &b, value)
		if err != nil {
			subErrors = append(subErrors, err)
		}
	}

	if len(subErrors) > 0 {
		return "", errSerialiseSubstitutions(substitutionContext, subErrors)
	}

	return b.String(), nil
}

func writeStringOrSubstitution(substitutionContext string, b *strings.Builder, value *StringOrSubstitution) error {
	if value.StringValue != nil {
		b.WriteString(*value.StringValue)
	} else {
		substitutionStr, err := SubstitutionToString(substitutionContext, value.SubstitutionValue)
		if err != nil {
			return err
		}
		b.WriteString("${")
		b.WriteString(substitutionStr)
		b.WriteString("}")

	}
	return nil
}

// SubstitutionToString converts a representation of a substitution with the ${..} syntax
// to a string.
func SubstitutionToString(substitutionContext string, substitution *Substitution) (string, error) {
	if substitution.Function != nil {
		return subFunctionToString(substitutionContext, substitution.Function)
	} else if substitution.Variable != nil {
		return subVariableToString(substitution.Variable)
	} else if substitution.DataSourceProperty != nil {
		return subDataSourcePropertyToString(substitution.DataSourceProperty)
	} else if substitution.ResourceProperty != nil {
		return SubResourcePropertyToString(substitution.ResourceProperty)
	} else if substitution.Child != nil {
		return subChildToString(substitution.Child)
	}
	return "", nil
}

func subFunctionToString(substitutionContext string, function *SubstitutionFunctionExpr) (string, error) {
	var b strings.Builder

	b.WriteString(string(function.FunctionName))
	b.WriteString("(")

	subErrors := []error{}
	for i, arg := range function.Arguments {
		err := writeFunctionArgument(substitutionContext, &b, arg)
		if err != nil {
			subErrors = append(subErrors, err)
		}

		if i < len(function.Arguments)-1 {
			b.WriteString(",")
		}
	}

	if len(subErrors) > 0 {
		return "", errSerialiseSubstitutions(substitutionContext, subErrors)
	}

	b.WriteString(")")
	return b.String(), nil
}

func writeFunctionArgument(substitutionContext string, b *strings.Builder, arg *SubstitutionFunctionArg) error {
	if arg.Value == nil {
		return errSerialiseSubstitutionFunctionArgValueMissing()
	}

	if arg.Name != "" {
		b.WriteString(fmt.Sprintf("%s = ", arg.Name))
	}

	if arg.Value.StringValue != nil {
		// String literals in the context of a function call
		// are always wrapped in double quotes.
		b.WriteString(fmt.Sprintf("\"%s\"", *arg.Value.StringValue))
	} else if arg.Value.IntValue != nil {
		b.WriteString(fmt.Sprintf("%d", *arg.Value.IntValue))
	} else if arg.Value.FloatValue != nil {
		b.WriteString(fmt.Sprintf("%f", *arg.Value.FloatValue))
	} else if arg.Value.BoolValue != nil {
		b.WriteString(fmt.Sprintf("%t", *arg.Value.BoolValue))
	} else {
		substitutionStr, err := SubstitutionToString(substitutionContext, arg.Value)
		if err != nil {
			return err
		}
		b.WriteString(substitutionStr)
	}

	return nil
}

func subVariableToString(variable *SubstitutionVariable) (string, error) {
	if NamePattern.MatchString(variable.VariableName) {
		return fmt.Sprintf("variables.%s", variable.VariableName), nil
	}

	if NameStringLiteralPattern.MatchString(variable.VariableName) {
		return fmt.Sprintf("variables[\"%s\"]", variable.VariableName), nil
	}

	return "", errSerialiseSubstitutionInvalidVariableName(variable.VariableName)
}

func subDataSourcePropertyToString(prop *SubstitutionDataSourceProperty) (string, error) {
	path := "datasources"
	if NamePattern.MatchString(prop.DataSourceName) {
		path += fmt.Sprintf(".%s", prop.DataSourceName)
	} else if NameStringLiteralPattern.MatchString(prop.DataSourceName) {
		path += fmt.Sprintf("[\"%s\"]", prop.DataSourceName)
	} else {
		return "", errSerialiseSubstitutionInvalidDataSourceName(prop.DataSourceName)
	}

	if NamePattern.MatchString(prop.FieldName) {
		path += fmt.Sprintf(".%s", prop.FieldName)
	} else if NameStringLiteralPattern.MatchString(prop.FieldName) {
		path += fmt.Sprintf("[\"%s\"]", prop.FieldName)
	} else {
		return "", errSerialiseSubstitutionInvalidDataSourcePath(prop.FieldName, prop.DataSourceName)
	}

	if prop.PrimitiveArrIndex != nil {
		path += fmt.Sprintf("[%d]", *prop.PrimitiveArrIndex)
	}

	return path, nil
}

// SubResourcePropertyToString produces a string representation of a substitution
// component that refers to a resource property.
func SubResourcePropertyToString(prop *SubstitutionResourceProperty) (string, error) {
	path := "resources"
	if NamePattern.MatchString(prop.ResourceName) {
		path += fmt.Sprintf(".%s", prop.ResourceName)
	} else if NameStringLiteralPattern.MatchString(prop.ResourceName) {
		path += fmt.Sprintf("[\"%s\"]", prop.ResourceName)
	} else {
		return "", errSerialiseSubstitutionInvalidResourceName(prop.ResourceName)
	}

	errors := []error{}
	rawPath := ""
	for _, pathItem := range prop.Path {
		pathItemStr, err := propertyPathItemToString(pathItem)
		if err != nil {
			errors = append(errors, err)
		} else {
			path += pathItemStr
		}
		rawPath += pathItemStr
	}

	if len(errors) > 0 {
		return "", errSerialiseSubstitutionInvalidChildPath(rawPath, prop.ResourceName, errors)
	}

	return path, nil
}

func propertyPathItemToString(pathItem *SubstitutionPathItem) (string, error) {
	if NamePattern.MatchString(pathItem.FieldName) {
		return fmt.Sprintf(".%s", pathItem.FieldName), nil
	} else if NameStringLiteralPattern.MatchString(pathItem.FieldName) {
		return fmt.Sprintf("[\"%s\"]", pathItem.FieldName), nil
	} else if pathItem.ArrayIndex != nil {
		return fmt.Sprintf("[%d]", *pathItem.ArrayIndex), nil
	}

	// Return the raw path item string so it can be used in higher level error messages.
	return fmt.Sprintf("[\"%s\"]", pathItem.FieldName), errSerialiseSubstitutionInvalidPathItem(pathItem)
}

func subChildToString(child *SubstitutionChild) (string, error) {
	path := "children"
	if NamePattern.MatchString(child.ChildName) {
		path += fmt.Sprintf(".%s", child.ChildName)
	} else if NameStringLiteralPattern.MatchString(child.ChildName) {
		path += fmt.Sprintf("[\"%s\"]", child.ChildName)
	} else {
		return "", errSerialiseSubstitutionInvalidChildName(child.ChildName)
	}

	if len(child.Path) == 0 {
		return "", errSerialiseSubstitutionInvalidChildPath("", child.ChildName, []error{})
	}

	errors := []error{}
	rawPath := ""
	for _, pathItem := range child.Path {
		pathItemStr, err := propertyPathItemToString(pathItem)
		if err != nil {
			errors = append(errors, err)
		} else {
			path += pathItemStr
		}
		rawPath += pathItemStr
	}

	if len(errors) > 0 {
		return "", errSerialiseSubstitutionInvalidChildPath(rawPath, child.ChildName, errors)
	}

	return path, nil
}

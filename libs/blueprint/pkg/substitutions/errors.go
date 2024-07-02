package substitutions

import (
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/errors"
)

const (
	// ErrorReasonCodeInvalidReferenceSub is provided when the reason
	// for a blueprint spec load error is due to one or more reference substitutions
	// being invalid.
	ErrorReasonCodeInvalidReferenceSub errors.ErrorReasonCode = "invalid_reference_substitution"

	childErrorsFormatStr = "\n\t- %s"
)

func errSubstitutions(
	substitutionContext string,
	substitutionErrors []error,
	line *int,
	column *int,
) error {
	return &errors.LoadError{
		ReasonCode:  ErrorReasonCodeInvalidReferenceSub,
		Err:         errSubstitutionsMessage(substitutionContext),
		ChildErrors: substitutionErrors,
		Line:        line,
		Column:      column,
	}
}

func errSerialiseSubstitutions(
	substitutionContext string,
	substitutionErrors []error,
) error {
	return &errors.SerialiseError{
		ReasonCode:  ErrorReasonCodeInvalidReferenceSub,
		Err:         errSubstitutionsMessage(substitutionContext),
		ChildErrors: substitutionErrors,
	}
}

func errSubstitutionsMessage(substitutionContext string) error {
	if len(substitutionContext) > 0 {
		return fmt.Errorf(
			"validation failed due to one or more invalid reference substitutions having been provided for \"%s\"",
			substitutionContext,
		)
	}

	return fmt.Errorf(
		"validation failed due to one or more invalid reference substitutions having been provided",
	)
}

func errSerialiseSubstitutionUnsupportedFunction(
	functionName SubstitutionFunctionName,
) error {
	return &errors.SerialiseError{
		ReasonCode: ErrorReasonCodeInvalidReferenceSub,
		Err: fmt.Errorf(
			"validation failed due to unsupported function \"%s\" having been provided in a reference substitution",
			functionName,
		),
	}
}

func errSerialiseSubstitutionInvalidVariableName(
	variableName string,
) error {
	return &errors.SerialiseError{
		ReasonCode: ErrorReasonCodeInvalidReferenceSub,
		Err: fmt.Errorf(
			"validation failed due to invalid variable name \"%s\" having been provided in a reference substitution",
			variableName,
		),
	}
}

func errSerialiseSubstitutionInvalidDataSourceName(
	dataSourceName string,
) error {
	return &errors.SerialiseError{
		ReasonCode: ErrorReasonCodeInvalidReferenceSub,
		Err: fmt.Errorf(
			"validation failed due to invalid data source name \"%s\" having been provided in a reference substitution",
			dataSourceName,
		),
	}
}

func errSerialiseSubstitutionInvalidDataSourcePath(
	path string,
	dataSourceName string,
) error {
	return &errors.SerialiseError{
		ReasonCode: ErrorReasonCodeInvalidReferenceSub,
		Err: fmt.Errorf(
			"validation failed due to invalid data source path \"%s\" for data source \"%s\" having been provided in a reference substitution",
			path,
			dataSourceName,
		),
	}
}

func errSerialiseSubstitutionInvalidResourceName(
	resourceName string,
) error {
	return &errors.SerialiseError{
		ReasonCode: ErrorReasonCodeInvalidReferenceSub,
		Err: fmt.Errorf(
			"validation failed due to invalid resource name \"%s\" having been provided in a reference substitution",
			resourceName,
		),
	}
}

func errSerialiseSubstitutionInvalidChildName(
	childName string,
) error {
	return &errors.SerialiseError{
		ReasonCode: ErrorReasonCodeInvalidReferenceSub,
		Err: fmt.Errorf(
			"validation failed due to invalid child blueprint name \"%s\" having been provided in a reference substitution",
			childName,
		),
	}
}

func errSerialiseSubstitutionInvalidChildPath(
	path string,
	childName string,
	pathItemErrors []error,
) error {
	childErrStr := ""
	for _, err := range pathItemErrors {
		childErrStr += fmt.Sprintf(childErrorsFormatStr, err.Error())
	}

	if len(path) == 0 {
		return &errors.SerialiseError{
			ReasonCode: ErrorReasonCodeInvalidReferenceSub,
			Err: fmt.Errorf(
				"validation failed due to an empty path having been provided for "+
					"child blueprint \"%s\" in a reference substitution, an exported field must be specified%s",
				childName,
				childErrStr,
			),
		}
	}

	return &errors.LoadError{
		ReasonCode: ErrorReasonCodeInvalidReferenceSub,
		Err: fmt.Errorf(
			"validation failed due to invalid path \"%s\" for child blueprint \"%s\" having been provided in a reference substitution%s",
			path,
			childName,
			childErrStr,
		),
	}
}

func errSerialiseSubstitutionInvalidPathItem(pathItem *SubstitutionPathItem) error {
	if pathItem.PrimitiveArrIndex == nil {
		return &errors.SerialiseError{
			ReasonCode: ErrorReasonCodeInvalidReferenceSub,
			Err: fmt.Errorf(
				"validation failed due to invalid path item \"%s\" having been provided in a reference substitution",
				pathItem.FieldName,
			),
		}
	}

	return &errors.SerialiseError{
		ReasonCode: ErrorReasonCodeInvalidReferenceSub,
		Err: fmt.Errorf(
			"validation failed due to invalid index accessor path item [\"%d\"] having been provided in a reference substitution",
			*pathItem.PrimitiveArrIndex,
		),
	}
}

func errLexUnexpectedEndOfInput(evaluatingTokenType string) error {
	return fmt.Errorf(
		"validation failed due to an unexpected end of input having "+
			"been encountered in a reference substitution when evaluating \"%s\"",
		evaluatingTokenType,
	)
}

func errLexUnexpectedChar(
	column int,
	char rune,
) error {
	return fmt.Errorf(
		"validation failed due to an unexpected character \"%s\" having "+
			"been encountered in a reference substitution at column %d",
		string(char),
		column,
	)
}

// ParseError is an error that is returned
// during failure to parse a reference substitution.
type ParseError struct {
	pos     int
	token   *token
	message string
	Line    int
	Column  int
}

func (e *ParseError) Error() string {
	var errStr string
	if e.token != nil {
		errStr = fmt.Sprintf("parse error at column %d with token type %s: %s", e.pos, e.token.tokenType, e.message)
	} else {
		errStr = fmt.Sprintf("parse error at end of input: %s", e.message)
	}
	return errStr
}

func errParseError(t *token, pos int, message string, line int, col int) error {
	return &ParseError{
		token:   t,
		pos:     pos,
		message: message,
		Line:    line,
		Column:  col,
	}
}

type ParseErrors struct {
	message string
	errors  []error
}

func (e *ParseErrors) Error() string {
	errStr := fmt.Sprintf("parse errors: %s", e.message)
	for _, err := range e.errors {
		errStr += fmt.Sprintf(childErrorsFormatStr, err.Error())
	}
	return errStr
}

func errParseErrorMultiple(message string, errors []error) error {
	return &ParseErrors{
		message: message,
		errors:  errors,
	}
}

type LexErrors struct {
	message string
	errors  []error
}

func (e *LexErrors) Error() string {
	errStr := fmt.Sprintf("lex errors: %s", e.message)
	for _, err := range e.errors {
		errStr += fmt.Sprintf(childErrorsFormatStr, err.Error())
	}
	return errStr
}

func errLexMultiple(message string, errors []error) error {
	return &LexErrors{
		message: message,
		errors:  errors,
	}
}

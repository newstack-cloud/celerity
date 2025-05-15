package core

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/source"
)

// ConfigDefinition contains a detailed definition (schema) of the configuration
// required for a provider or transformer plugin.
// Fields that have dynamic keys should use the `<placeholder>` syntax
// in the key name, e.g. "aws.config.regionKMSKeys.<region>".
// The value that "<placeholder>" represents must be a string that
// matches the pattern [A-Za-z0-9\-_]+.
// Schema validation will match based on the pattern for dynamic keys.
// Dynamic keys are often useful to emulate nested dynamic map or array structures
// in provider and transformer configuration.
// Default values are ignored for config field definitions that have
// dynamic field names.
type ConfigDefinition struct {
	Fields                map[string]*ConfigFieldDefinition `json:"fields"`
	AllowAdditionalFields bool                              `json:"allowAdditionalFields"`
}

// ConfigFieldDefinition represents a field in a configuration definition
// for a provider or transformer plugin.
type ConfigFieldDefinition struct {
	Type        ScalarType `json:"type"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	// Set to true if the config field should be a secret,
	// An application such as the deploy engine
	// will ensure that the value of this field
	// does not appear in logs.
	Secret bool `json:"secret"`
	// If the name of this field represents a config value template
	// (e.g. "aws.config.regionKMSKeys.<region>"),
	// the default value will not be used and is ignored in validation.
	DefaultValue  *ScalarValue   `json:"defaultValue,omitempty"`
	AllowedValues []*ScalarValue `json:"allowedValues,omitempty"`
	Examples      []*ScalarValue `json:"examples,omitempty"`
	Required      bool           `json:"required"`
}

// PopulateDefaultConfigValues populates the default values
// for the given configuration based on the provided config definition.
// This is used to ensure that the default values are set
// for the configuration before it is used.
func PopulateDefaultConfigValues(
	config map[string]*ScalarValue,
	configDefinition *ConfigDefinition,
) (map[string]*ScalarValue, error) {
	configWithDefaults := map[string]*ScalarValue{}
	maps.Copy(configWithDefaults, config)

	for fieldDefName, fieldDef := range configDefinition.Fields {
		configValues, isDynamicFieldName, err := getConfigValues(fieldDefName, config)
		if err != nil {
			return nil, err
		}
		// Default values are ignored for dynamic field names
		// that match multiple config values.
		if !isDynamicFieldName && len(configValues) == 0 {
			if !IsScalarNil(fieldDef.DefaultValue) {
				configWithDefaults[fieldDefName] = fieldDef.DefaultValue
			}
		}
	}

	return configWithDefaults, nil
}

// ValidateConfigDefinition checks provider and transformer
// configuration against a given config definition schema.
// This is intended to be used before change staging, deploy
// and destroy actions for an instance of a blueprint.
// This validation supports dynamic field names that match
// a config field name that contains one or more "<placeholder>"
// strings.
// The value that "<placeholder>" represents must be a string that
// matches the pattern [A-Za-z0-9\-_]+.
// For example, a config field name of "aws.config.regionKMSKeys.<region>"
// would match a config field name of "aws.config.regionKMSKeys.us-east-1"
// and "aws.config.regionKMSKeys.us-west-2".
// When a field definition with a dynamic name is required, it means that
// at least one config value that matches the pattern must be present.
// Default values are ignored for config field definitions that have
// dynamic field names.
// This returns an error for any unexpected errors and will return
// a list of diagnostics for any validation errors and warnings.
func ValidateConfigDefinition(
	pluginName string,
	pluginType string,
	config map[string]*ScalarValue,
	configDefinition *ConfigDefinition,
) ([]*Diagnostic, error) {
	diagnostics := []*Diagnostic{}
	matchedFieldNames := []string{}

	for fieldDefName, fieldDef := range configDefinition.Fields {
		// Collect multiple config values to account for field definitions
		// that have dynamic field names that can match multiple config values.
		configValues, isDynamicFieldName, err := getConfigValues(fieldDefName, config)
		if err != nil {
			return diagnostics, err
		}
		matchedFieldNames = addUniqueFieldNames(matchedFieldNames, configValues)

		// A default value is only used if the field is not a template
		// for multiple config values.
		hasDefault := !IsScalarNil(fieldDef.DefaultValue)
		isDefaultValueSet := hasDefault && !isDynamicFieldName

		if fieldDef.Required &&
			!isDefaultValueSet &&
			len(configValues) == 0 {
			diagnostics = append(diagnostics, &Diagnostic{
				Level: DiagnosticLevelError,
				Message: missingRequiredFieldMessage(
					pluginName,
					pluginType,
					fieldDefName,
					isDynamicFieldName,
				),
				Range: generalDiagnosticRange(),
			})
		}

		if len(fieldDef.AllowedValues) > 0 {
			allowedValueDiagnostics := []*Diagnostic{}
			checkAllowedConfigValues(
				pluginName,
				pluginType,
				configValues,
				fieldDef.AllowedValues,
				&allowedValueDiagnostics,
			)
			diagnostics = append(diagnostics, allowedValueDiagnostics...)
		}
	}

	if configDefinition.AllowAdditionalFields {
		return diagnostics, nil
	}

	for configName := range config {
		if !slices.Contains(matchedFieldNames, configName) {
			diagnostics = append(diagnostics, &Diagnostic{
				Level: DiagnosticLevelError,
				Message: fmt.Sprintf(
					"The %q %s configuration contains an unexpected field %q.",
					pluginName,
					pluginType,
					configName,
				),
				Range: generalDiagnosticRange(),
			})
		}
	}

	return diagnostics, nil
}

func checkAllowedConfigValues(
	pluginName string,
	pluginType string,
	configValues map[string]*ScalarValue,
	allowedValues []*ScalarValue,
	diagnostics *[]*Diagnostic,
) {
	for configName, configValue := range configValues {
		if !slices.ContainsFunc(
			allowedValues,
			func(allowedValue *ScalarValue) bool {
				return configValue.Equal(allowedValue)
			},
		) {
			*diagnostics = append(*diagnostics, &Diagnostic{
				Level: DiagnosticLevelError,
				Message: fmt.Sprintf(
					"The %q %s configuration field %q has an unexpected value %s.",
					pluginName,
					pluginType,
					configName,
					configValue.ToString(),
				),
				Range: generalDiagnosticRange(),
			})
		}
	}
}

func missingRequiredFieldMessage(
	pluginName string,
	pluginType string,
	fieldDefName string,
	dynamicFieldName bool,
) string {
	if dynamicFieldName {
		return fmt.Sprintf(
			"The %q %s configuration requires at least one config "+
				"value with a key that matches the pattern %q.",
			pluginName,
			pluginType,
			fieldDefName,
		)
	}

	return fmt.Sprintf(
		"The %q %s configuration requires the field %q.",
		pluginName,
		pluginType,
		fieldDefName,
	)
}

func getConfigValues(
	fieldDefName string,
	config map[string]*ScalarValue,
) (map[string]*ScalarValue, bool, error) {
	configValues := map[string]*ScalarValue{}

	if !isDynamicConfigFieldName(fieldDefName) {
		if configValue, ok := config[fieldDefName]; ok {
			configValues[fieldDefName] = configValue
		}
		return configValues, false, nil
	}

	patternString := createPatternForDynamicFieldName(fieldDefName)
	pattern, err := regexp.Compile(patternString)
	if err != nil {
		return nil, false, err
	}
	for configName, configValue := range config {
		if pattern.MatchString(configName) {
			configValues[configName] = configValue
		}
	}

	return configValues, true, nil
}

func isDynamicConfigFieldName(fieldDefName string) bool {
	indexOpenAngleBracket := strings.Index(fieldDefName, "<")
	indexCloseAngleBracket := strings.Index(fieldDefName, ">")
	return indexOpenAngleBracket != -1 &&
		indexCloseAngleBracket != -1 &&
		indexOpenAngleBracket < indexCloseAngleBracket
}

const (
	capturePlaceholderPatternString = "([A-Za-z0-9\\-_]+)"
)

func createPatternForDynamicFieldName(fieldDefName string) string {
	startIndex := 0
	for startIndex != -1 {
		searchIn := fieldDefName[startIndex:]
		openIndex := strings.Index(searchIn, "<")
		closeIndex := strings.Index(searchIn, ">")
		if openIndex == -1 || closeIndex == -1 {
			startIndex = -1
		} else {
			fieldDefName = fieldDefName[:startIndex+openIndex] +
				capturePlaceholderPatternString +
				fieldDefName[startIndex+closeIndex+1:]
			startIndex += closeIndex + 1
		}

	}
	return fieldDefName
}

func addUniqueFieldNames(
	fieldNames []string,
	matchedConfigValues map[string]*ScalarValue,
) []string {
	for configName := range matchedConfigValues {
		if !slices.Contains(fieldNames, configName) {
			fieldNames = append(fieldNames, configName)
		}
	}

	return fieldNames
}

// To be used when there is no position information available
// for elements in a source file. (e.g. provider or transformer config values)
func generalDiagnosticRange() *DiagnosticRange {
	return &DiagnosticRange{
		Start: &source.Meta{
			Position: source.Position{
				Line:   1,
				Column: 1,
			},
		},
		End: &source.Meta{
			Position: source.Position{
				Line:   1,
				Column: 1,
			},
		},
	}
}

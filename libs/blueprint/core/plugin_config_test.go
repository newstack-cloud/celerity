package core

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/suite"
)

type PluginConfigTestSuite struct {
	suite.Suite
}

func (s *PluginConfigTestSuite) Test_populate_defaults_for_missing_config_values() {
	inputConfig := map[string]*ScalarValue{
		"intField":    ScalarFromInt(45),
		"floatField":  ScalarFromFloat(3.14),
		"boolField":   ScalarFromBool(true),
		"stringField": ScalarFromString("a value"),
		// Include dynamic field to make sure it is ignored for
		// populating default values.
		"aws.config.regionKMSKeys.us-east-1.other.value1": ScalarFromString(
			"arn:aws:kms:us-east-1:123456789012:key/abcd1234",
		),
	}
	configWithDefaults, err := PopulateDefaultConfigValues(
		inputConfig,
		testConfigDefinition,
	)
	s.Assert().NoError(err)

	expectedConfig := map[string]*ScalarValue{}
	maps.Copy(expectedConfig, inputConfig)
	expectedConfig["intFieldWithDefault"] = ScalarFromInt(100)

	s.Assert().Equal(expectedConfig, configWithDefaults)
}

func (s *PluginConfigTestSuite) Test_passes_validation_for_valid_input_config() {
	inputConfig := map[string]*ScalarValue{
		"intField":    ScalarFromInt(10),
		"floatField":  ScalarFromFloat(3.14),
		"boolField":   ScalarFromBool(true),
		"stringField": ScalarFromString("another value"),
		// Dynamic fields based on the
		// aws.config.regionKMSKeys.<region>.other.<placeholder>
		// "template" in the config definition.
		"aws.config.regionKMSKeys.us-east-1.other.value1": ScalarFromString(
			"arn:aws:kms:us-east-1:123456789012:key/abcd1234",
		),
		"aws.config.regionKMSKeys.eu-west-1.other.value2": ScalarFromString(
			"arn:aws:kms:eu-west-1:123456789012:key/abcd2345",
		),
	}
	diagnostics, err := ValidateConfigDefinition(
		"aws",
		"provider",
		inputConfig,
		testConfigDefinition,
	)
	s.Assert().NoError(err)
	s.Assert().Empty(diagnostics)
}

func (s *PluginConfigTestSuite) Test_passes_validation_for_valid_input_config_that_allows_additional_values() {
	inputConfig := map[string]*ScalarValue{
		"intField":    ScalarFromInt(10),
		"floatField":  ScalarFromFloat(3.14),
		"boolField":   ScalarFromBool(true),
		"stringField": ScalarFromString("another value"),
		// Dynamic fields based on the
		// aws.config.regionKMSKeys.<region>.other.<placeholder>
		// "template" in the config definition.
		"aws.config.regionKMSKeys.us-east-1.other.value1": ScalarFromString(
			"arn:aws:kms:us-east-1:123456789012:key/abcd1234",
		),
		"aws.config.regionKMSKeys.eu-west-1.other.value2": ScalarFromString(
			"arn:aws:kms:eu-west-1:123456789012:key/abcd2345",
		),
		"additionalField": ScalarFromString("additional value"),
	}
	diagnostics, err := ValidateConfigDefinition(
		"aws",
		"provider",
		inputConfig,
		testConfigDefinitionWithAdditionalValues(),
	)
	s.Assert().NoError(err)
	s.Assert().Empty(diagnostics)
}

func (s *PluginConfigTestSuite) Test_fails_validation_for_missing_required_value() {
	inputConfig := map[string]*ScalarValue{
		"intField":   ScalarFromInt(10),
		"floatField": ScalarFromFloat(3.14),
		// "boolField" is missing
		"stringField": ScalarFromString("another value"),
		// Dynamic fields based on the
		// aws.config.regionKMSKeys.<region>.other.<placeholder>
		// "template" in the config definition.
		"aws.config.regionKMSKeys.us-east-1.other.value1": ScalarFromString(
			"arn:aws:kms:us-east-1:123456789012:key/abcd1234",
		),
		"aws.config.regionKMSKeys.eu-west-1.other.value2": ScalarFromString(
			"arn:aws:kms:eu-west-1:123456789012:key/abcd2345",
		),
	}
	diagnostics, err := ValidateConfigDefinition(
		"aws",
		"provider",
		inputConfig,
		testConfigDefinition,
	)
	s.Assert().NoError(err)
	s.Assert().Equal(
		[]*Diagnostic{
			{
				Level:   DiagnosticLevelError,
				Message: "The \"aws\" provider configuration requires the field \"boolField\".",
				Range:   generalDiagnosticRange(),
			},
		},
		diagnostics,
	)
}

func (s *PluginConfigTestSuite) Test_fails_validation_for_missing_dynamic_fields() {
	inputConfig := map[string]*ScalarValue{
		"intField":    ScalarFromInt(10),
		"floatField":  ScalarFromFloat(3.14),
		"boolField":   ScalarFromBool(true),
		"stringField": ScalarFromString("another value"),
		// At least one dynamic field value is required.
	}
	diagnostics, err := ValidateConfigDefinition(
		"aws",
		"provider",
		inputConfig,
		testConfigDefinition,
	)
	s.Assert().NoError(err)
	s.Assert().Equal(
		[]*Diagnostic{
			{
				Level: DiagnosticLevelError,
				Message: "The \"aws\" provider configuration requires at least one config " +
					"value with a key that matches the pattern " +
					"\"aws.config.regionKMSKeys.<region>.other.<placeholder>\".",
				Range: generalDiagnosticRange(),
			},
		},
		diagnostics,
	)
}

func (s *PluginConfigTestSuite) Test_fails_validation_when_additional_values_are_not_allowed() {
	inputConfig := map[string]*ScalarValue{
		"intField":    ScalarFromInt(10),
		"floatField":  ScalarFromFloat(3.14),
		"boolField":   ScalarFromBool(true),
		"stringField": ScalarFromString("another value"),
		// Dynamic fields based on the
		// aws.config.regionKMSKeys.<region>.other.<placeholder>
		// "template" in the config definition.
		"aws.config.regionKMSKeys.us-east-1.other.value1": ScalarFromString(
			"arn:aws:kms:us-east-1:123456789012:key/abcd1234",
		),
		"aws.config.regionKMSKeys.eu-west-1.other.value2": ScalarFromString(
			"arn:aws:kms:eu-west-1:123456789012:key/abcd2345",
		),
		"additionalField": ScalarFromString("additional value"),
	}
	diagnostics, err := ValidateConfigDefinition(
		"aws",
		"provider",
		inputConfig,
		testConfigDefinition,
	)
	s.Assert().NoError(err)
	s.Assert().Equal(
		[]*Diagnostic{
			{
				Level: DiagnosticLevelError,
				Message: "The \"aws\" provider configuration contains " +
					"an unexpected field \"additionalField\".",
				Range: generalDiagnosticRange(),
			},
		},
		diagnostics,
	)
}

func (s *PluginConfigTestSuite) Test_fails_validation_for_values_not_in_allowed_list() {
	inputConfig := map[string]*ScalarValue{
		// 70 is not in the allowed values list.
		"intField":    ScalarFromInt(70),
		"floatField":  ScalarFromFloat(3.14),
		"boolField":   ScalarFromBool(true),
		"stringField": ScalarFromString("another value"),
		// Dynamic fields based on the
		// aws.config.regionKMSKeys.<region>.other.<placeholder>
		// "template" in the config definition.
		"aws.config.regionKMSKeys.us-east-1.other.value1": ScalarFromString(
			"arn:aws:kms:us-east-1:123456789012:key/abcd1234",
		),
		"aws.config.regionKMSKeys.eu-west-1.other.value2": ScalarFromString(
			"arn:aws:kms:eu-west-1:123456789012:key/abcd2345",
		),
	}
	diagnostics, err := ValidateConfigDefinition(
		"aws",
		"provider",
		inputConfig,
		testConfigDefinition,
	)
	s.Assert().NoError(err)
	s.Assert().Equal(
		[]*Diagnostic{
			{
				Level: DiagnosticLevelError,
				Message: "The \"aws\" provider configuration field " +
					"\"intField\" has an unexpected value 70.",
				Range: generalDiagnosticRange(),
			},
		},
		diagnostics,
	)
}

func (s *PluginConfigTestSuite) Test_fails_validation_for_value_of_wrong_type() {
	inputConfig := map[string]*ScalarValue{
		// intField is expected to be an integer value.
		"intField":    ScalarFromString("not an integer"),
		"floatField":  ScalarFromFloat(3.14),
		"boolField":   ScalarFromBool(true),
		"stringField": ScalarFromString("another value"),
		// Dynamic fields based on the
		// aws.config.regionKMSKeys.<region>.other.<placeholder>
		// "template" in the config definition.
		"aws.config.regionKMSKeys.us-east-1.other.value1": ScalarFromString(
			"arn:aws:kms:us-east-1:123456789012:key/abcd1234",
		),
		"aws.config.regionKMSKeys.eu-west-1.other.value2": ScalarFromString(
			"arn:aws:kms:eu-west-1:123456789012:key/abcd2345",
		),
	}
	diagnostics, err := ValidateConfigDefinition(
		"aws",
		"provider",
		inputConfig,
		testConfigDefinition,
	)
	s.Assert().NoError(err)
	s.Assert().Equal(
		[]*Diagnostic{
			{
				Level: DiagnosticLevelError,
				Message: "The value of the \"intField\" config field in the " +
					"aws provider is not a valid integer. Expected a value of type integer, but got string.",
				Range: generalDiagnosticRange(),
			},
			{
				Level: DiagnosticLevelError,
				Message: "The \"aws\" provider configuration field \"intField\" " +
					"has an unexpected value not an integer.",
				Range: generalDiagnosticRange(),
			},
		},
		diagnostics,
	)
}

func TestPluginConfigTestSuite(t *testing.T) {
	suite.Run(t, new(PluginConfigTestSuite))
}

var testConfigDefinition = &ConfigDefinition{
	Fields: map[string]*ConfigFieldDefinition{
		"intField": {
			Type:        ScalarTypeInteger,
			Label:       "Int Field",
			Description: "An integer field",
			AllowedValues: []*ScalarValue{
				ScalarFromInt(10),
				ScalarFromInt(22),
				ScalarFromInt(45),
				ScalarFromInt(57),
			},
			Required: true,
		},
		"floatField": {
			Type:        ScalarTypeFloat,
			Label:       "Float Field",
			Description: "A float field",
			Required:    true,
		},
		"boolField": {
			Type:        ScalarTypeBool,
			Label:       "Bool Field",
			Description: "A boolean field",
			Required:    true,
		},
		"stringField": {
			Type:        ScalarTypeString,
			Label:       "String Field",
			Description: "A string field",
			Required:    true,
		},
		"aws.config.regionKMSKeys.<region>.other.<placeholder>": {
			Type:        ScalarTypeString,
			Label:       "AWS Region KMS Keys",
			Description: "AWS region KMS keys",
			Required:    true,
		},
		"intFieldWithDefault": {
			Type:         ScalarTypeInteger,
			Label:        "Int Field with Default",
			Description:  "An integer field with a default value",
			DefaultValue: ScalarFromInt(100),
		},
	},
}

func testConfigDefinitionWithAdditionalValues() *ConfigDefinition {
	fields := map[string]*ConfigFieldDefinition{}
	maps.Copy(fields, testConfigDefinition.Fields)
	return &ConfigDefinition{
		AllowAdditionalFields: true,
		Fields:                fields,
	}
}

package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v3"
)

func Test(t *testing.T) {
	TestingT(t)
}

type VariableTestSuite struct {
	specFixtures map[string][]byte
}

var _ = Suite(&VariableTestSuite{})

func (s *VariableTestSuite) SetUpSuite(c *C) {
	s.specFixtures = make(map[string][]byte)
	fixturesToLoad := map[string]string{
		"passYAML":                        "__testdata/variables/pass.yml",
		"failUnsupportedVariableTypeYAML": "__testdata/variables/fail-unsupported-variable-type.yml",
		"serialiseExpectedYAML":           "__testdata/variables/serialise-expected.yml",
		"passJSON":                        "__testdata/variables/pass.json",
		"failUnsupportedVariableTypeJSON": "__testdata/variables/fail-unsupported-variable-type.json",
		"serialiseExpectedJSON":           "__testdata/variables/serialise-expected.json",
	}

	for name, filePath := range fixturesToLoad {
		specBytes, err := os.ReadFile(filePath)
		if err != nil {
			c.Error(err)
			c.FailNow()
		}
		s.specFixtures[name] = specBytes
	}
}

func (s *VariableTestSuite) Test_parses_valid_variable_yaml_input(c *C) {
	targetVar := &Variable{}
	err := yaml.Unmarshal([]byte(s.specFixtures["passYAML"]), targetVar)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(*targetVar.Default.BoolValue, Equals, true)
	c.Assert(targetVar.Description, Equals, "This is an example boolean variable")
	c.Assert(targetVar.Secret, Equals, false)
	c.Assert(targetVar.Type.Value, Equals, VariableType("boolean"))
}

func (s *VariableTestSuite) Test_fails_to_parse_yaml_due_to_unsupported_variable_type(c *C) {
	targetVar := &Variable{}
	err := yaml.Unmarshal([]byte(s.specFixtures["failUnsupportedVariableTypeYAML"]), targetVar)
	if err == nil {
		c.Error(errors.New("expected to fail deserialisation due to unsupported variable type"))
		c.FailNow()
	}

	schemaError, isSchemaError := err.(*Error)
	c.Assert(isSchemaError, Equals, true)
	c.Assert(schemaError.ReasonCode, Equals, ErrorSchemaReasonCodeInvalidVariableType)
}

func (s *VariableTestSuite) Test_serialise_valid_variable_yaml_input(c *C) {
	expected := &Variable{}
	err := yaml.Unmarshal([]byte(s.specFixtures["serialiseExpectedYAML"]), expected)
	if err != nil {
		c.Error(fmt.Errorf("failed to parse expected fixture to compare with: %s", err.Error()))
		c.FailNow()
	}

	region := "eu-west-2"
	serialisedBytes, err := yaml.Marshal(&Variable{
		Type: &VariableTypeWrapper{
			Value: VariableTypeString,
		},
		Description: "The AWS region to connect to AWS services with",
		Secret:      false,
		Default: &core.ScalarValue{
			StringValue: &region,
		},
	})
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	targetVar := &Variable{}
	err = yaml.Unmarshal(serialisedBytes, targetVar)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetVar.Type.Value, Equals, expected.Type.Value)
	c.Assert(targetVar.Description, Equals, expected.Description)
	c.Assert(targetVar.Secret, Equals, expected.Secret)
	c.Assert(*targetVar.Default.StringValue, Equals, *expected.Default.StringValue)
}

func (s *VariableTestSuite) Test_fails_to_serialise_yaml_due_to_unsupported_variable_type(c *C) {
	_, err := yaml.Marshal(&Variable{
		Type: &VariableTypeWrapper{
			// "object" is not a valid variable type.
			Value: VariableType("object"),
		},
		Description: "The AWS region to connect to AWS services with",
		Secret:      false,
	})
	if err == nil {
		c.Error(errors.New("expected to fail serialisation due to unsupported variable type"))
		c.FailNow()
	}

	schemaError, isSchemaError := err.(*Error)
	c.Assert(isSchemaError, Equals, true)
	c.Assert(schemaError.ReasonCode, Equals, ErrorSchemaReasonCodeInvalidVariableType)
}

func (s *VariableTestSuite) Test_parses_valid_variable_json_input(c *C) {
	targetVar := &Variable{}
	err := json.Unmarshal([]byte(s.specFixtures["passJSON"]), targetVar)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(*targetVar.Default.IntValue, Equals, 3423)
	c.Assert(targetVar.Description, Equals, "This is an example integer variable")
	c.Assert(targetVar.Secret, Equals, false)
	c.Assert(targetVar.Type.Value, Equals, VariableType("integer"))
}

func (s *VariableTestSuite) Test_fails_to_parse_json_due_to_unsupported_variable_type(c *C) {
	targetVar := &Variable{}
	err := json.Unmarshal([]byte(s.specFixtures["failUnsupportedVariableTypeJSON"]), targetVar)
	if err == nil {
		c.Error(errors.New("expected to fail deserialisation due to unsupported variable type"))
		c.FailNow()
	}

	schemaError, isSchemaError := err.(*Error)
	c.Assert(isSchemaError, Equals, true)
	c.Assert(schemaError.ReasonCode, Equals, ErrorSchemaReasonCodeInvalidVariableType)
}

func (s *VariableTestSuite) Test_serialise_valid_variable_json_input(c *C) {
	expected := &Variable{}
	err := json.Unmarshal([]byte(s.specFixtures["serialiseExpectedJSON"]), expected)
	if err != nil {
		c.Error(fmt.Errorf("failed to parse expected fixture to compare with: %s", err.Error()))
		c.FailNow()
	}

	region := "eu-west-1"
	serialisedBytes, err := json.Marshal(&Variable{
		Type: &VariableTypeWrapper{
			Value: VariableTypeString,
		},
		Description: "The AWS region to connect to AWS services with",
		Secret:      true,
		Default: &core.ScalarValue{
			StringValue: &region,
		},
	})
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	targetVar := &Variable{}
	err = json.Unmarshal(serialisedBytes, targetVar)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetVar.Type.Value, Equals, expected.Type.Value)
	c.Assert(targetVar.Description, Equals, expected.Description)
	c.Assert(targetVar.Secret, Equals, expected.Secret)
	c.Assert(*targetVar.Default.StringValue, Equals, *expected.Default.StringValue)
}

func (s *VariableTestSuite) Test_fails_to_serialise_json_due_to_unsupported_variable_type(c *C) {
	_, err := json.Marshal(&Variable{
		Type: &VariableTypeWrapper{
			// "list" is not a valid variable type.
			Value: VariableType("list"),
		},
		Description: "The AWS region to connect to AWS services with",
		Secret:      false,
	})
	if err == nil {
		c.Error(errors.New("expected to fail serialisation due to unsupported variable type"))
		c.FailNow()
	}

	marshalError, isMarshalError := err.(*json.MarshalerError)
	c.Assert(isMarshalError, Equals, true)
	internalError := marshalError.Unwrap()

	schemaError, isSchemaError := internalError.(*Error)
	c.Assert(isSchemaError, Equals, true)
	c.Assert(schemaError.ReasonCode, Equals, ErrorSchemaReasonCodeInvalidVariableType)
}

package core

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
)

type ScalarTestSuite struct {
	specParseFixtures     map[string][]byte
	specSerialiseFixtures *ScalarValue
	suite.Suite
}

func (s *ScalarTestSuite) SetupTest() {
	s.specParseFixtures = map[string][]byte{
		"stringValYAML":    []byte("Test string value"),
		"stringValJSON":    []byte("\"Test string value\""),
		"intVal":           []byte("45172131"),
		"boolVal":          []byte("true"),
		"float64Val":       []byte("340239.3019484858723489"),
		"failInvalidValue": []byte("[\"invalid\",\"value\"]"),
	}
	intVal := 6509321
	boolVal := true
	stringVal := "Test serialise string"
	floatVal := 4509232.4032
	s.specSerialiseFixtures = &ScalarValue{
		IntValue:    &intVal,
		BoolValue:   &boolVal,
		StringValue: &stringVal,
		FloatValue:  &floatVal,
	}
}

func (s *ScalarTestSuite) Test_parse_string_value_yaml() {
	targetScalar := &ScalarValue{}
	err := yaml.Unmarshal([]byte(s.specParseFixtures["stringValYAML"]), targetScalar)
	s.Require().NoError(err)

	s.Assert().Equal(string(s.specParseFixtures["stringValYAML"]), *targetScalar.StringValue)
	s.Assert().Nil(targetScalar.BoolValue)
	s.Assert().Nil(targetScalar.IntValue)
	s.Assert().Nil(targetScalar.FloatValue)
	s.Assert().Equal(targetScalar.SourceMeta.Line, 1)
	s.Assert().Equal(targetScalar.SourceMeta.Column, 1)
}

func (s *ScalarTestSuite) Test_parse_int_value_yaml() {
	targetScalar := &ScalarValue{}
	err := yaml.Unmarshal([]byte(s.specParseFixtures["intVal"]), targetScalar)
	s.Require().NoError(err)

	intVal, err := strconv.Atoi(string(s.specParseFixtures["intVal"]))
	s.Require().NoError(err)
	s.Assert().Equal(intVal, *targetScalar.IntValue)
	s.Assert().Nil(targetScalar.BoolValue)
	s.Assert().Nil(targetScalar.StringValue)
	s.Assert().Nil(targetScalar.FloatValue)
}

func (s *ScalarTestSuite) Test_parse_bool_value_yaml() {
	targetScalar := &ScalarValue{}
	err := yaml.Unmarshal([]byte(s.specParseFixtures["boolVal"]), targetScalar)
	s.Require().NoError(err)

	boolVal, err := strconv.ParseBool(string(s.specParseFixtures["boolVal"]))
	s.Require().NoError(err)
	s.Assert().Equal(boolVal, *targetScalar.BoolValue)
	s.Assert().Nil(targetScalar.IntValue)
	s.Assert().Nil(targetScalar.StringValue)
	s.Assert().Nil(targetScalar.FloatValue)
}

func (s *ScalarTestSuite) Test_parse_float64_value_yaml() {
	targetScalar := &ScalarValue{}
	err := yaml.Unmarshal([]byte(s.specParseFixtures["float64Val"]), targetScalar)
	s.Require().NoError(err)

	float64Val, err := strconv.ParseFloat(string(s.specParseFixtures["float64Val"]), 64)
	s.Require().NoError(err)

	s.Assert().Equal(float64Val, *targetScalar.FloatValue)
	s.Assert().Nil(targetScalar.IntValue)
	s.Assert().Nil(targetScalar.StringValue)
	s.Assert().Nil(targetScalar.BoolValue)
}

func (s *ScalarTestSuite) Test_parse_fails_for_invalid_value_yaml() {
	targetScalar := &ScalarValue{}
	err := yaml.Unmarshal([]byte(s.specParseFixtures["failInvalidValue"]), targetScalar)
	s.Require().NotNil(err)

	coreErr, isCoreError := err.(*Error)
	s.Require().True(isCoreError)
	s.Assert().Equal(ErrorCoreReasonCodeMustBeScalar, coreErr.ReasonCode)
	s.Assert().Equal(1, *coreErr.SourceLine)
	s.Assert().Equal(1, *coreErr.SourceColumn)
}

func (s *ScalarTestSuite) Test_serialise_string_value_yaml() {
	// New line is added to yaml parsing output by the yaml library.
	expected := fmt.Sprintf("%s\n", *s.specSerialiseFixtures.StringValue)
	toSerialise := &ScalarValue{
		StringValue: s.specSerialiseFixtures.StringValue,
	}
	serialised, err := yaml.Marshal(toSerialise)
	s.Require().NoError(err)
	s.Assert().Equal(expected, string(serialised))
}

func (s *ScalarTestSuite) Test_serialise_int_value_yaml() {
	// New line is added to yaml parsing output by the yaml library.
	expected := fmt.Sprintf("%d\n", *s.specSerialiseFixtures.IntValue)
	toSerialise := &ScalarValue{
		IntValue: s.specSerialiseFixtures.IntValue,
	}
	serialised, err := yaml.Marshal(toSerialise)
	s.Require().NoError(err)

	s.Assert().Equal(expected, string(serialised))
}

func (s *ScalarTestSuite) Test_serialise_bool_value_yaml() {
	// New line is added to yaml parsing output by the yaml library.
	expected := fmt.Sprintf("%t\n", *s.specSerialiseFixtures.BoolValue)
	toSerialise := &ScalarValue{
		BoolValue: s.specSerialiseFixtures.BoolValue,
	}
	serialised, err := yaml.Marshal(toSerialise)
	s.Require().NoError(err)
	s.Assert().Equal(expected, string(serialised))
}

func (s *ScalarTestSuite) Test_serialise_float64_value_yaml() {
	// New line is added to yaml parsing output by the yaml library.
	// The yaml library uses exponents when marshalling floating point numbers
	// so we must match the format.
	// ('e' for exponent and precision of 10 to include all digits)
	floatStr := strconv.FormatFloat(*s.specSerialiseFixtures.FloatValue, 'e', 10, 64)
	expected := fmt.Sprintf("%s\n", floatStr)
	toSerialise := &ScalarValue{
		FloatValue: s.specSerialiseFixtures.FloatValue,
	}
	serialised, err := yaml.Marshal(toSerialise)
	s.Require().NoError(err)

	s.Assert().Equal(expected, string(serialised))
}

func (s *ScalarTestSuite) Test_parse_string_value_json() {
	targetScalar := &ScalarValue{}
	err := json.Unmarshal([]byte(s.specParseFixtures["stringValJSON"]), targetScalar)
	s.Require().NoError(err)

	fixtureStr := string(s.specParseFixtures["stringValJSON"])
	expectedWithoutQuotes := fixtureStr[1 : len(fixtureStr)-1]
	s.Assert().Equal(expectedWithoutQuotes, *targetScalar.StringValue)
	s.Assert().Nil(targetScalar.BoolValue)
	s.Assert().Nil(targetScalar.IntValue)
	s.Assert().Nil(targetScalar.FloatValue)
}

func (s *ScalarTestSuite) Test_parse_int_value_json() {
	targetScalar := &ScalarValue{}
	err := json.Unmarshal([]byte(s.specParseFixtures["intVal"]), targetScalar)
	s.Require().NoError(err)

	intVal, err := strconv.Atoi(string(s.specParseFixtures["intVal"]))
	s.Require().NoError(err)

	s.Assert().Equal(intVal, *targetScalar.IntValue)
	s.Assert().Nil(targetScalar.BoolValue)
	s.Assert().Nil(targetScalar.StringValue)
	s.Assert().Nil(targetScalar.FloatValue)
}

func (s *ScalarTestSuite) Test_parse_bool_value_json() {
	targetScalar := &ScalarValue{}
	err := json.Unmarshal([]byte(s.specParseFixtures["boolVal"]), targetScalar)
	s.Require().NoError(err)

	boolVal, err := strconv.ParseBool(string(s.specParseFixtures["boolVal"]))
	s.Require().NoError(err)
	s.Assert().Equal(boolVal, *targetScalar.BoolValue)
	s.Assert().Nil(targetScalar.IntValue)
	s.Assert().Nil(targetScalar.StringValue)
	s.Assert().Nil(targetScalar.FloatValue)
}

func (s *ScalarTestSuite) Test_parse_float64_value_json() {
	targetScalar := &ScalarValue{}
	err := json.Unmarshal([]byte(s.specParseFixtures["float64Val"]), targetScalar)
	s.Require().NoError(err)

	float64Val, err := strconv.ParseFloat(string(s.specParseFixtures["float64Val"]), 64)
	s.Require().NoError(err)

	s.Assert().Equal(float64Val, *targetScalar.FloatValue)
	s.Assert().Nil(targetScalar.IntValue)
	s.Assert().Nil(targetScalar.StringValue)
	s.Assert().Nil(targetScalar.BoolValue)
}

func (s *ScalarTestSuite) Test_parse_fails_for_invalid_value_json() {
	targetScalar := &ScalarValue{}
	err := json.Unmarshal([]byte(s.specParseFixtures["failInvalidValue"]), targetScalar)
	s.Require().NotNil(err)

	s.Assert().Equal(errMustBeScalar(nil).Error(), err.Error())
}

func (s *ScalarTestSuite) Test_serialise_string_value_json() {
	// New line is added to yaml parsing output by the yaml library.
	expected := fmt.Sprintf("\"%s\"", *s.specSerialiseFixtures.StringValue)
	toSerialise := &ScalarValue{
		StringValue: s.specSerialiseFixtures.StringValue,
	}
	serialised, err := json.Marshal(toSerialise)
	s.Require().NoError(err)

	s.Assert().Equal(expected, string(serialised))
}

func (s *ScalarTestSuite) Test_serialise_int_value_json() {
	// New line is added to yaml parsing output by the yaml library.
	expected := fmt.Sprintf("%d", *s.specSerialiseFixtures.IntValue)
	toSerialise := &ScalarValue{
		IntValue: s.specSerialiseFixtures.IntValue,
	}
	serialised, err := json.Marshal(toSerialise)
	s.Require().NoError(err)

	s.Assert().Equal(expected, string(serialised))
}

func (s *ScalarTestSuite) Test_serialise_bool_value_json() {
	// New line is added to yaml parsing output by the yaml library.
	expected := fmt.Sprintf("%t", *s.specSerialiseFixtures.BoolValue)
	toSerialise := &ScalarValue{
		BoolValue: s.specSerialiseFixtures.BoolValue,
	}
	serialised, err := json.Marshal(toSerialise)
	s.Require().NoError(err)

	s.Assert().Equal(expected, string(serialised))
}

func (s *ScalarTestSuite) Test_serialise_float64_value_json() {
	// New line is added to yaml parsing output by the yaml library.
	// The json library uses decimal format without exponents so we need to match
	// it for our expected serialised value.
	expected := strconv.FormatFloat(*s.specSerialiseFixtures.FloatValue, 'f', 4, 64)
	toSerialise := &ScalarValue{
		FloatValue: s.specSerialiseFixtures.FloatValue,
	}
	serialised, err := json.Marshal(toSerialise)
	s.Require().NoError(err)

	s.Assert().Equal(expected, string(serialised))
}

func TestScalarTestSuite(t *testing.T) {
	suite.Run(t, new(ScalarTestSuite))
}

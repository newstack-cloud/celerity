package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"

	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v3"
)

func Test(t *testing.T) {
	TestingT(t)
}

type ScalarTestSuite struct {
	specParseFixtures     map[string][]byte
	specSerialiseFixtures *ScalarValue
}

var _ = Suite(&ScalarTestSuite{})

func (s *ScalarTestSuite) SetUpSuite(c *C) {
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

func (s *ScalarTestSuite) Test_parse_string_value_yaml(c *C) {
	targetScalar := &ScalarValue{}
	err := yaml.Unmarshal([]byte(s.specParseFixtures["stringValYAML"]), targetScalar)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(*targetScalar.StringValue, Equals, string(s.specParseFixtures["stringValYAML"]))
	c.Assert(targetScalar.BoolValue, IsNil)
	c.Assert(targetScalar.IntValue, IsNil)
	c.Assert(targetScalar.FloatValue, IsNil)
}

func (s *ScalarTestSuite) Test_parse_int_value_yaml(c *C) {
	targetScalar := &ScalarValue{}
	err := yaml.Unmarshal([]byte(s.specParseFixtures["intVal"]), targetScalar)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	intVal, err := strconv.Atoi(string(s.specParseFixtures["intVal"]))
	if err != nil {
		c.Error(err)
		c.FailNow()
	}
	c.Assert(*targetScalar.IntValue, Equals, intVal)
	c.Assert(targetScalar.BoolValue, IsNil)
	c.Assert(targetScalar.StringValue, IsNil)
	c.Assert(targetScalar.FloatValue, IsNil)
}

func (s *ScalarTestSuite) Test_parse_bool_value_yaml(c *C) {
	targetScalar := &ScalarValue{}
	err := yaml.Unmarshal([]byte(s.specParseFixtures["boolVal"]), targetScalar)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	boolVal, err := strconv.ParseBool(string(s.specParseFixtures["boolVal"]))
	if err != nil {
		c.Error(err)
		c.FailNow()
	}
	c.Assert(*targetScalar.BoolValue, Equals, boolVal)
	c.Assert(targetScalar.IntValue, IsNil)
	c.Assert(targetScalar.StringValue, IsNil)
	c.Assert(targetScalar.FloatValue, IsNil)
}

func (s *ScalarTestSuite) Test_parse_float64_value_yaml(c *C) {
	targetScalar := &ScalarValue{}
	err := yaml.Unmarshal([]byte(s.specParseFixtures["float64Val"]), targetScalar)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	float64Val, err := strconv.ParseFloat(string(s.specParseFixtures["float64Val"]), 64)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(*targetScalar.FloatValue, Equals, float64Val)
	c.Assert(targetScalar.IntValue, IsNil)
	c.Assert(targetScalar.StringValue, IsNil)
	c.Assert(targetScalar.BoolValue, IsNil)
}

func (s *ScalarTestSuite) Test_parse_fails_for_invalid_value_yaml(c *C) {
	targetScalar := &ScalarValue{}
	err := yaml.Unmarshal([]byte(s.specParseFixtures["failInvalidValue"]), targetScalar)
	if err == nil {
		c.Error(errors.New("expected to fail due to a non-scalar value being provided"))
		c.FailNow()
	}

	c.Assert(err, Equals, ErrValueMustBeScalar)
}

func (s *ScalarTestSuite) Test_serialise_string_value_yaml(c *C) {
	// New line is added to yaml parsing output by the yaml library.
	expected := fmt.Sprintf("%s\n", *s.specSerialiseFixtures.StringValue)
	toSerialise := &ScalarValue{
		StringValue: s.specSerialiseFixtures.StringValue,
	}
	serialised, err := yaml.Marshal(toSerialise)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(string(serialised), Equals, expected)
}

func (s *ScalarTestSuite) Test_serialise_int_value_yaml(c *C) {
	// New line is added to yaml parsing output by the yaml library.
	expected := fmt.Sprintf("%d\n", *s.specSerialiseFixtures.IntValue)
	toSerialise := &ScalarValue{
		IntValue: s.specSerialiseFixtures.IntValue,
	}
	serialised, err := yaml.Marshal(toSerialise)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(string(serialised), Equals, expected)
}

func (s *ScalarTestSuite) Test_serialise_bool_value_yaml(c *C) {
	// New line is added to yaml parsing output by the yaml library.
	expected := fmt.Sprintf("%t\n", *s.specSerialiseFixtures.BoolValue)
	toSerialise := &ScalarValue{
		BoolValue: s.specSerialiseFixtures.BoolValue,
	}
	serialised, err := yaml.Marshal(toSerialise)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(string(serialised), Equals, expected)
}

func (s *ScalarTestSuite) Test_serialise_float64_value_yaml(c *C) {
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
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(string(serialised), Equals, expected)
}

func (s *ScalarTestSuite) Test_parse_string_value_json(c *C) {
	targetScalar := &ScalarValue{}
	err := json.Unmarshal([]byte(s.specParseFixtures["stringValJSON"]), targetScalar)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	fixtureStr := string(s.specParseFixtures["stringValJSON"])
	expectedWithoutQuotes := fixtureStr[1 : len(fixtureStr)-1]
	c.Assert(*targetScalar.StringValue, Equals, expectedWithoutQuotes)
	c.Assert(targetScalar.BoolValue, IsNil)
	c.Assert(targetScalar.IntValue, IsNil)
	c.Assert(targetScalar.FloatValue, IsNil)
}

func (s *ScalarTestSuite) Test_parse_int_value_json(c *C) {
	targetScalar := &ScalarValue{}
	err := json.Unmarshal([]byte(s.specParseFixtures["intVal"]), targetScalar)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	intVal, err := strconv.Atoi(string(s.specParseFixtures["intVal"]))
	if err != nil {
		c.Error(err)
		c.FailNow()
	}
	c.Assert(*targetScalar.IntValue, Equals, intVal)
	c.Assert(targetScalar.BoolValue, IsNil)
	c.Assert(targetScalar.StringValue, IsNil)
	c.Assert(targetScalar.FloatValue, IsNil)
}

func (s *ScalarTestSuite) Test_parse_bool_value_json(c *C) {
	targetScalar := &ScalarValue{}
	err := json.Unmarshal([]byte(s.specParseFixtures["boolVal"]), targetScalar)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	boolVal, err := strconv.ParseBool(string(s.specParseFixtures["boolVal"]))
	if err != nil {
		c.Error(err)
		c.FailNow()
	}
	c.Assert(*targetScalar.BoolValue, Equals, boolVal)
	c.Assert(targetScalar.IntValue, IsNil)
	c.Assert(targetScalar.StringValue, IsNil)
	c.Assert(targetScalar.FloatValue, IsNil)
}

func (s *ScalarTestSuite) Test_parse_float64_value_json(c *C) {
	targetScalar := &ScalarValue{}
	err := json.Unmarshal([]byte(s.specParseFixtures["float64Val"]), targetScalar)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	float64Val, err := strconv.ParseFloat(string(s.specParseFixtures["float64Val"]), 64)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(*targetScalar.FloatValue, Equals, float64Val)
	c.Assert(targetScalar.IntValue, IsNil)
	c.Assert(targetScalar.StringValue, IsNil)
	c.Assert(targetScalar.BoolValue, IsNil)
}

func (s *ScalarTestSuite) Test_parse_fails_for_invalid_value_json(c *C) {
	targetScalar := &ScalarValue{}
	err := json.Unmarshal([]byte(s.specParseFixtures["failInvalidValue"]), targetScalar)
	if err == nil {
		c.Error(errors.New("expected to fail due to a non-scalar value being provided"))
		c.FailNow()
	}

	c.Assert(err, Equals, ErrValueMustBeScalar)
}

func (s *ScalarTestSuite) Test_serialise_string_value_json(c *C) {
	// New line is added to yaml parsing output by the yaml library.
	expected := fmt.Sprintf("\"%s\"", *s.specSerialiseFixtures.StringValue)
	toSerialise := &ScalarValue{
		StringValue: s.specSerialiseFixtures.StringValue,
	}
	serialised, err := json.Marshal(toSerialise)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(string(serialised), Equals, expected)
}

func (s *ScalarTestSuite) Test_serialise_int_value_json(c *C) {
	// New line is added to yaml parsing output by the yaml library.
	expected := fmt.Sprintf("%d", *s.specSerialiseFixtures.IntValue)
	toSerialise := &ScalarValue{
		IntValue: s.specSerialiseFixtures.IntValue,
	}
	serialised, err := json.Marshal(toSerialise)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(string(serialised), Equals, expected)
}

func (s *ScalarTestSuite) Test_serialise_bool_value_json(c *C) {
	// New line is added to yaml parsing output by the yaml library.
	expected := fmt.Sprintf("%t", *s.specSerialiseFixtures.BoolValue)
	toSerialise := &ScalarValue{
		BoolValue: s.specSerialiseFixtures.BoolValue,
	}
	serialised, err := json.Marshal(toSerialise)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(string(serialised), Equals, expected)
}

func (s *ScalarTestSuite) Test_serialise_float64_value_json(c *C) {
	// New line is added to yaml parsing output by the yaml library.
	// The json library uses decimal format without exponents so we need to match
	// it for our expected serialised value.
	expected := strconv.FormatFloat(*s.specSerialiseFixtures.FloatValue, 'f', 4, 64)
	toSerialise := &ScalarValue{
		FloatValue: s.specSerialiseFixtures.FloatValue,
	}
	serialised, err := json.Marshal(toSerialise)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(string(serialised), Equals, expected)
}

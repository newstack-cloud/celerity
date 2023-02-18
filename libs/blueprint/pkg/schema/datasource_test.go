package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v3"
)

type DataSourceFieldExportTestSuite struct {
	specFixtures map[string][]byte
}

var _ = Suite(&DataSourceFieldExportTestSuite{})

func (s *DataSourceFieldExportTestSuite) SetUpSuite(c *C) {
	s.specFixtures = make(map[string][]byte)
	fixturesToLoad := map[string]string{
		"passYAML":                               "__testdata/datasourcefields/pass.yml",
		"failUnsupportedDataSourceFieldTypeYAML": "__testdata/datasourcefields/fail-unsupported-data-source-field-type.yml",
		"serialiseExpectedYAML":                  "__testdata/datasourcefields/serialise-expected.yml",
		"passJSON":                               "__testdata/datasourcefields/pass.json",
		"failUnsupportedDataSourceFieldTypeJSON": "__testdata/datasourcefields/fail-unsupported-data-source-field-type.json",
		"serialiseExpectedJSON":                  "__testdata/datasourcefields/serialise-expected.json",
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

func (s *DataSourceFieldExportTestSuite) Test_parses_valid_data_source_field_yaml_input(c *C) {
	targetField := &DataSourceFieldExport{}
	err := yaml.Unmarshal([]byte(s.specFixtures["passYAML"]), targetField)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetField.Description, Equals, "This is an example boolean data source field")
	c.Assert(targetField.Type.Value, Equals, DataSourceFieldType("boolean"))
}

func (s *DataSourceFieldExportTestSuite) Test_fails_to_parse_yaml_due_to_unsupported_data_source_field_type(c *C) {
	targetField := &DataSourceFieldExport{}
	err := yaml.Unmarshal([]byte(s.specFixtures["failUnsupportedDataSourceFieldTypeYAML"]), targetField)
	if err == nil {
		c.Error(errors.New("expected to fail deserialisation due to unsupported data source field type"))
		c.FailNow()
	}

	schemaError, isSchemaError := err.(*Error)
	c.Assert(isSchemaError, Equals, true)
	c.Assert(schemaError.ReasonCode, Equals, ErrorSchemaReasonCodeInvalidDataSourceFieldType)
}

func (s *DataSourceFieldExportTestSuite) Test_serialise_valid_data_source_field_yaml_input(c *C) {
	expected := &DataSourceFieldExport{}
	err := yaml.Unmarshal([]byte(s.specFixtures["serialiseExpectedYAML"]), expected)
	if err != nil {
		c.Error(fmt.Errorf("failed to parse expected fixture to compare with: %s", err.Error()))
		c.FailNow()
	}

	serialisedBytes, err := yaml.Marshal(&DataSourceFieldExport{
		Type: &DataSourceFieldTypeWrapper{
			Value: DataSourceFieldTypeString,
		},
		Description: "The AWS region to connect to AWS services with",
	})
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	targetField := &DataSourceFieldExport{}
	err = yaml.Unmarshal(serialisedBytes, targetField)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetField.Type.Value, Equals, expected.Type.Value)
	c.Assert(targetField.Description, Equals, expected.Description)
}

func (s *DataSourceFieldExportTestSuite) Test_fails_to_serialise_yaml_due_to_unsupported_data_source_type(c *C) {
	_, err := yaml.Marshal(&DataSourceFieldExport{
		Type: &DataSourceFieldTypeWrapper{
			// "unknown" is not a valid variable type.
			Value: DataSourceFieldType("unknown"),
		},
		Description: "The AWS region to connect to AWS services with",
	})
	if err == nil {
		c.Error(errors.New("expected to fail serialisation due to unsupported data source field type"))
		c.FailNow()
	}

	schemaError, isSchemaError := err.(*Error)
	c.Assert(isSchemaError, Equals, true)
	c.Assert(schemaError.ReasonCode, Equals, ErrorSchemaReasonCodeInvalidDataSourceFieldType)
}

func (s *DataSourceFieldExportTestSuite) Test_parses_valid_data_source_field_json_input(c *C) {
	targetField := &DataSourceFieldExport{}
	err := json.Unmarshal([]byte(s.specFixtures["passJSON"]), targetField)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetField.Description, Equals, "This is an example integer data source field")
	c.Assert(targetField.Type.Value, Equals, DataSourceFieldType("integer"))
}

func (s *DataSourceFieldExportTestSuite) Test_fails_to_parse_json_due_to_unsupported_data_source_type(c *C) {
	targetField := &DataSourceFieldExport{}
	err := json.Unmarshal([]byte(s.specFixtures["failUnsupportedDataSourceFieldTypeJSON"]), targetField)
	if err == nil {
		c.Error(errors.New("expected to fail deserialisation due to unsupported data source field type"))
		c.FailNow()
	}

	schemaError, isSchemaError := err.(*Error)
	c.Assert(isSchemaError, Equals, true)
	c.Assert(schemaError.ReasonCode, Equals, ErrorSchemaReasonCodeInvalidDataSourceFieldType)
}

func (s *DataSourceFieldExportTestSuite) Test_serialise_valid_data_source_field_json_input(c *C) {
	expected := &DataSourceFieldExport{}
	err := json.Unmarshal([]byte(s.specFixtures["serialiseExpectedJSON"]), expected)
	if err != nil {
		c.Error(fmt.Errorf("failed to parse expected fixture to compare with: %s", err.Error()))
		c.FailNow()
	}

	serialisedBytes, err := json.Marshal(&DataSourceFieldExport{
		Type: &DataSourceFieldTypeWrapper{
			Value: DataSourceFieldTypeString,
		},
		Description: "The AWS region to connect to AWS services with",
	})
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	targetField := &DataSourceFieldExport{}
	err = json.Unmarshal(serialisedBytes, targetField)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetField.Type.Value, Equals, expected.Type.Value)
	c.Assert(targetField.Description, Equals, expected.Description)
}

func (s *DataSourceFieldExportTestSuite) Test_fails_to_serialise_json_due_to_unsupported_data_source_field_type(c *C) {
	_, err := json.Marshal(&DataSourceFieldExport{
		Type: &DataSourceFieldTypeWrapper{
			// "list" is not a valid data source field type.
			Value: DataSourceFieldType("list"),
		},
		Description: "The AWS region to connect to AWS services with",
	})
	if err == nil {
		c.Error(errors.New("expected to fail serialisation due to unsupported data source field type"))
		c.FailNow()
	}

	marshalError, isMarshalError := err.(*json.MarshalerError)
	c.Assert(isMarshalError, Equals, true)
	internalError := marshalError.Unwrap()

	schemaError, isSchemaError := internalError.(*Error)
	c.Assert(isSchemaError, Equals, true)
	c.Assert(schemaError.ReasonCode, Equals, ErrorSchemaReasonCodeInvalidDataSourceFieldType)
}

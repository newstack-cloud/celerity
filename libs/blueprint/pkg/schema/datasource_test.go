package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/substitutions"
	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v3"
)

type DataSourceTestSuite struct {
	specFixtures map[string][]byte
}

var _ = Suite(&DataSourceTestSuite{})

func (s *DataSourceTestSuite) SetUpSuite(c *C) {
	s.specFixtures = make(map[string][]byte)
	fixturesToLoad := map[string]string{
		"fields-passYAML": "__testdata/datasourcefields/pass.yml",
		"fields-failUnsupportedDataSourceFieldTypeYAML":       "__testdata/datasourcefields/fail-unsupported-data-source-field-type.yml",
		"fields-serialiseExpectedYAML":                        "__testdata/datasourcefields/serialise-expected.yml",
		"fields-passJSON":                                     "__testdata/datasourcefields/pass.json",
		"fields-failUnsupportedDataSourceFieldTypeJSON":       "__testdata/datasourcefields/fail-unsupported-data-source-field-type.json",
		"fields-serialiseExpectedJSON":                        "__testdata/datasourcefields/serialise-expected.json",
		"filters-passYAML":                                    "__testdata/datasourcefilters/pass.yml",
		"filters-failUnsupportedDataSourceFilterOperatorYAML": "__testdata/datasourcefilters/fail-unsupported-data-source-filter-operator.yml",
		"filters-serialiseExpectedYAML":                       "__testdata/datasourcefilters/serialise-expected.yml",
		"filters-passJSON":                                    "__testdata/datasourcefilters/pass.json",
		"filters-failUnsupportedDataSourceFilterOperatorJSON": "__testdata/datasourcefilters/fail-unsupported-data-source-filter-operator.json",
		"filters-serialiseExpectedJSON":                       "__testdata/datasourcefilters/serialise-expected.json",
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

func (s *DataSourceTestSuite) Test_parses_valid_data_source_field_yaml_input(c *C) {
	targetField := &DataSourceFieldExport{}
	err := yaml.Unmarshal([]byte(s.specFixtures["fields-passYAML"]), targetField)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetField.Description, Equals, "This is an example boolean data source field")
	c.Assert(targetField.Type.Value, Equals, DataSourceFieldType("boolean"))
}

func (s *DataSourceTestSuite) Test_fails_to_parse_yaml_due_to_unsupported_data_source_field_type(c *C) {
	targetField := &DataSourceFieldExport{}
	err := yaml.Unmarshal([]byte(s.specFixtures["fields-failUnsupportedDataSourceFieldTypeYAML"]), targetField)
	if err == nil {
		c.Error(errors.New("expected to fail deserialisation due to unsupported data source field type"))
		c.FailNow()
	}

	schemaError, isSchemaError := err.(*Error)
	c.Assert(isSchemaError, Equals, true)
	c.Assert(schemaError.ReasonCode, Equals, ErrorSchemaReasonCodeInvalidDataSourceFieldType)
}

func (s *DataSourceTestSuite) Test_serialise_valid_data_source_field_yaml_input(c *C) {
	expected := &DataSourceFieldExport{}
	err := yaml.Unmarshal([]byte(s.specFixtures["fields-serialiseExpectedYAML"]), expected)
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

func (s *DataSourceTestSuite) Test_fails_to_serialise_yaml_due_to_unsupported_data_source_type(c *C) {
	_, err := yaml.Marshal(&DataSourceFieldExport{
		Type: &DataSourceFieldTypeWrapper{
			// "unknown" is not a valid data source field type.
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

func (s *DataSourceTestSuite) Test_parses_valid_data_source_field_json_input(c *C) {
	targetField := &DataSourceFieldExport{}
	err := json.Unmarshal([]byte(s.specFixtures["fields-passJSON"]), targetField)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetField.Description, Equals, "This is an example integer data source field")
	c.Assert(targetField.Type.Value, Equals, DataSourceFieldType("integer"))
}

func (s *DataSourceTestSuite) Test_fails_to_parse_json_due_to_unsupported_data_source_type(c *C) {
	targetField := &DataSourceFieldExport{}
	err := json.Unmarshal([]byte(s.specFixtures["fields-failUnsupportedDataSourceFieldTypeJSON"]), targetField)
	if err == nil {
		c.Error(errors.New("expected to fail deserialisation due to unsupported data source field type"))
		c.FailNow()
	}

	schemaError, isSchemaError := err.(*Error)
	c.Assert(isSchemaError, Equals, true)
	c.Assert(schemaError.ReasonCode, Equals, ErrorSchemaReasonCodeInvalidDataSourceFieldType)
}

func (s *DataSourceTestSuite) Test_serialise_valid_data_source_field_json_input(c *C) {
	expected := &DataSourceFieldExport{}
	err := json.Unmarshal([]byte(s.specFixtures["fields-serialiseExpectedJSON"]), expected)
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

func (s *DataSourceTestSuite) Test_fails_to_serialise_json_due_to_unsupported_data_source_field_type(c *C) {
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

func (s *DataSourceTestSuite) Test_parses_valid_data_source_filter_yaml_input(c *C) {
	targetFilter := &DataSourceFilter{}
	err := yaml.Unmarshal([]byte(s.specFixtures["filters-passYAML"]), targetFilter)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetFilter.Field, Equals, "tags")
	c.Assert(targetFilter.Operator.Value, Equals, DataSourceFilterOperatorHasKey)
	c.Assert(*targetFilter.Search.Values[0].Values[0].StringValue, Equals, "${variables.environment}")
}

func (s *DataSourceTestSuite) Test_fails_to_parse_yaml_due_to_unsupported_data_source_filter(c *C) {
	targetFilter := &DataSourceFilter{}
	err := yaml.Unmarshal([]byte(s.specFixtures["filters-failUnsupportedDataSourceFilterOperatorYAML"]), targetFilter)
	if err == nil {
		c.Error(errors.New("expected to fail deserialisation due to unsupported data source filter operator"))
		c.FailNow()
	}

	schemaError, isSchemaError := err.(*Error)
	c.Assert(isSchemaError, Equals, true)
	c.Assert(schemaError.ReasonCode, Equals, ErrorSchemaReasonCodeInvalidDataSourceFilterOperator)
}

func (s *DataSourceTestSuite) Test_serialise_valid_data_source_filter_yaml_input(c *C) {
	expected := &DataSourceFilter{}
	err := yaml.Unmarshal([]byte(s.specFixtures["filters-serialiseExpectedYAML"]), expected)
	if err != nil {
		c.Error(fmt.Errorf("failed to parse expected fixture to compare with: %s", err.Error()))
		c.FailNow()
	}

	searchFor := "ACTIVE"
	serialisedBytes, err := yaml.Marshal(&DataSourceFilter{
		Field: "configuration.status",
		Operator: &DataSourceFilterOperatorWrapper{
			Value: DataSourceFilterOperatorEquals,
		},
		Search: &DataSourceFilterSearch{
			Values: []*substitutions.StringOrSubstitutions{
				{
					Values: []*substitutions.StringOrSubstitution{
						{
							StringValue: &searchFor,
						},
					},
				},
			},
		},
	})
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	targetFilter := &DataSourceFilter{}
	err = yaml.Unmarshal(serialisedBytes, targetFilter)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetFilter.Field, Equals, expected.Field)
	c.Assert(targetFilter.Operator.Value, Equals, expected.Operator.Value)
	c.Assert(*targetFilter.Search.Values[0].Values[0].StringValue, Equals, *expected.Search.Values[0].Values[0].StringValue)
}

func (s *DataSourceTestSuite) Test_fails_to_serialise_yaml_due_to_unsupported_data_source_filter_operator(c *C) {
	search := "test-"
	_, err := yaml.Marshal(&DataSourceFilter{
		Field: "name",
		Operator: &DataSourceFilterOperatorWrapper{
			// "unknown" is not a valid filter operator.
			Value: DataSourceFilterOperator("unknown"),
		},
		Search: &DataSourceFilterSearch{
			Values: []*substitutions.StringOrSubstitutions{
				{
					Values: []*substitutions.StringOrSubstitution{
						{
							StringValue: &search,
						},
					},
				},
			},
		},
	})
	if err == nil {
		c.Error(errors.New("expected to fail serialisation due to unsupported data source filter operator"))
		c.FailNow()
	}

	schemaError, isSchemaError := err.(*Error)
	c.Assert(isSchemaError, Equals, true)
	c.Assert(schemaError.ReasonCode, Equals, ErrorSchemaReasonCodeInvalidDataSourceFilterOperator)
}

func (s *DataSourceTestSuite) Test_parses_valid_data_source_filter_json_input(c *C) {
	targetFilter := &DataSourceFilter{}
	err := json.Unmarshal([]byte(s.specFixtures["filters-passJSON"]), targetFilter)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetFilter.Field, Equals, "tags")
	c.Assert(targetFilter.Operator.Value, Equals, DataSourceFilterOperatorHasKey)
	c.Assert(*targetFilter.Search.Values[0].Values[0].StringValue, Equals, "${variables.environment}")
}

func (s *DataSourceTestSuite) Test_fails_to_parse_json_due_to_unsupported_data_source_filter_operator(c *C) {
	targetFilter := &DataSourceFilter{}
	err := json.Unmarshal([]byte(s.specFixtures["filters-failUnsupportedDataSourceFilterOperatorJSON"]), targetFilter)
	if err == nil {
		c.Error(errors.New("expected to fail deserialisation due to unsupported data source filter operator"))
		c.FailNow()
	}

	schemaError, isSchemaError := err.(*Error)
	c.Assert(isSchemaError, Equals, true)
	c.Assert(schemaError.ReasonCode, Equals, ErrorSchemaReasonCodeInvalidDataSourceFilterOperator)
}

func (s *DataSourceTestSuite) Test_serialise_valid_data_source_filter_json_input(c *C) {
	expected := &DataSourceFilter{}
	err := json.Unmarshal([]byte(s.specFixtures["filters-serialiseExpectedJSON"]), expected)
	if err != nil {
		c.Error(fmt.Errorf("failed to parse expected fixture to compare with: %s", err.Error()))
		c.FailNow()
	}

	searchFor := "ACTIVE"
	serialisedBytes, err := yaml.Marshal(&DataSourceFilter{
		Field: "configuration.status",
		Operator: &DataSourceFilterOperatorWrapper{
			Value: DataSourceFilterOperatorEquals,
		},
		Search: &DataSourceFilterSearch{
			Values: []*substitutions.StringOrSubstitutions{
				{
					Values: []*substitutions.StringOrSubstitution{
						{
							StringValue: &searchFor,
						},
					},
				},
			},
		},
	})
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	targetFilter := &DataSourceFilter{}
	err = yaml.Unmarshal(serialisedBytes, targetFilter)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetFilter.Field, Equals, expected.Field)
	c.Assert(targetFilter.Operator.Value, Equals, expected.Operator.Value)
	c.Assert(*targetFilter.Search.Values[0].Values[0].StringValue, Equals, *expected.Search.Values[0].Values[0].StringValue)
}

func (s *DataSourceTestSuite) Test_fails_to_serialise_json_due_to_unsupported_data_source_filter_operator(c *C) {
	search := "test-"
	_, err := json.Marshal(&DataSourceFilter{
		Field: "name",
		Operator: &DataSourceFilterOperatorWrapper{
			// "unknown" is not a valid filter operator.
			Value: DataSourceFilterOperator("unknown"),
		},
		Search: &DataSourceFilterSearch{
			Values: []*substitutions.StringOrSubstitutions{
				{
					Values: []*substitutions.StringOrSubstitution{
						{
							StringValue: &search,
						},
					},
				},
			},
		},
	})
	if err == nil {
		c.Error(errors.New("expected to fail serialisation due to unsupported data source filter operator"))
		c.FailNow()
	}

	marshalError, isMarshalError := err.(*json.MarshalerError)
	c.Assert(isMarshalError, Equals, true)
	internalError := marshalError.Unwrap()

	schemaError, isSchemaError := internalError.(*Error)
	c.Assert(isSchemaError, Equals, true)
	c.Assert(schemaError.ReasonCode, Equals, ErrorSchemaReasonCodeInvalidDataSourceFilterOperator)
}

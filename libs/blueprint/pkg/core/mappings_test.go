package core

import (
	"encoding/json"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/source"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/substitutions"
	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v3"
)

var (
	testStringValue   = "Test string value"
	testStringValPart = "Test string value for "
)

type MappingNodeTestSuite struct {
	specParseFixtures     map[string][]byte
	specSerialiseFixtures map[string]*MappingNode
}

var _ = Suite(&MappingNodeTestSuite{})

func (s *MappingNodeTestSuite) SetUpSuite(c *C) {
	s.prepareParseInputFixtures()
	s.prepareExpectedFixtures()
}

func (s *MappingNodeTestSuite) Test_parse_string_val_yaml(c *C) {
	targetMappingNode := &MappingNode{}
	err := yaml.Unmarshal(s.specParseFixtures["stringValYAML"], targetMappingNode)
	c.Assert(err, IsNil)
	c.Assert(targetMappingNode.Literal.StringValue, NotNil)
	c.Assert(*targetMappingNode.Literal.StringValue, Equals, testStringValue)
	c.Assert(targetMappingNode.Literal.SourceMeta.Line, Equals, 1)
	c.Assert(targetMappingNode.Literal.SourceMeta.Column, Equals, 1)
}

func (s *MappingNodeTestSuite) Test_parse_string_val_json(c *C) {
	targetMappingNode := &MappingNode{}
	err := json.Unmarshal(s.specParseFixtures["stringValJSON"], targetMappingNode)
	c.Assert(err, IsNil)
	c.Assert(targetMappingNode.Literal.StringValue, NotNil)
	c.Assert(*targetMappingNode.Literal.StringValue, Equals, testStringValue)
}

func (s *MappingNodeTestSuite) Test_parse_string_with_subs_yaml(c *C) {
	targetMappingNode := &MappingNode{}
	err := yaml.Unmarshal(s.specParseFixtures["stringWithSubsYAML"], targetMappingNode)
	c.Assert(err, IsNil)
	c.Assert(targetMappingNode, DeepEquals, &MappingNode{
		StringWithSubstitutions: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{StringValue: &testStringValPart},
				{
					SubstitutionValue: &substitutions.Substitution{
						Variable: &substitutions.SubstitutionVariable{
							VariableName: "environment",
						},
					},
				},
			},
		},
	})
}

func (s *MappingNodeTestSuite) Test_parse_string_with_subs_json(c *C) {
	targetMappingNode := &MappingNode{}
	err := json.Unmarshal(s.specParseFixtures["stringWithSubsJSON"], targetMappingNode)
	c.Assert(err, IsNil)
	c.Assert(targetMappingNode, DeepEquals, &MappingNode{
		StringWithSubstitutions: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{StringValue: &testStringValPart},
				{
					SubstitutionValue: &substitutions.Substitution{
						Variable: &substitutions.SubstitutionVariable{
							VariableName: "environment",
						},
					},
				},
			},
		},
	})
}

func (s *MappingNodeTestSuite) Test_parse_int_val(c *C) {
	targetMappingNode := &MappingNode{}
	err := yaml.Unmarshal(s.specParseFixtures["intVal"], targetMappingNode)
	c.Assert(err, IsNil)
	c.Assert(targetMappingNode.Literal.IntValue, NotNil)
	c.Assert(*targetMappingNode.Literal.IntValue, Equals, 45172131)
}

func (s *MappingNodeTestSuite) Test_parse_fields_val_yaml(c *C) {
	targetMappingNode := &MappingNode{}
	err := yaml.Unmarshal(s.specParseFixtures["fieldsValYAML"], targetMappingNode)
	c.Assert(err, IsNil)
	assertFieldsNodeYAML(c, targetMappingNode)
}

func (s *MappingNodeTestSuite) Test_parse_fields_val_json(c *C) {
	targetMappingNode := &MappingNode{}
	err := json.Unmarshal(s.specParseFixtures["fieldsValJSON"], targetMappingNode)
	c.Assert(err, IsNil)
	assertFieldsNodeJSON(c, targetMappingNode)
}

func assertFieldsNodeYAML(c *C, actual *MappingNode) {
	expectedIntVal := 45172131
	expectedStrVal := "value1"
	expectedStrSubPrefix := "value with sub "
	c.Assert(actual, DeepEquals, &MappingNode{
		Fields: map[string]*MappingNode{
			"key1": {
				Literal: &ScalarValue{StringValue: &expectedStrVal, SourceMeta: &source.Meta{Line: 2, Column: 15}},
			},
			"key2": {
				StringWithSubstitutions: &substitutions.StringOrSubstitutions{
					Values: []*substitutions.StringOrSubstitution{
						{StringValue: &expectedStrSubPrefix},
						{
							SubstitutionValue: &substitutions.Substitution{
								Variable: &substitutions.SubstitutionVariable{
									VariableName: "environment",
								},
							},
						},
					},
				},
			},
			"key3": {
				Literal: &ScalarValue{IntValue: &expectedIntVal, SourceMeta: &source.Meta{Line: 4, Column: 15}},
			},
		},
	})
}

func assertFieldsNodeJSON(c *C, actual *MappingNode) {
	expectedIntVal := 45172131
	expectedStrVal := "value1"
	expectedStrSubPrefix := "value with sub "
	c.Assert(actual, DeepEquals, &MappingNode{
		Fields: map[string]*MappingNode{
			"key1": {
				Literal: &ScalarValue{StringValue: &expectedStrVal},
			},
			"key2": {
				StringWithSubstitutions: &substitutions.StringOrSubstitutions{
					Values: []*substitutions.StringOrSubstitution{
						{StringValue: &expectedStrSubPrefix},
						{
							SubstitutionValue: &substitutions.Substitution{
								Variable: &substitutions.SubstitutionVariable{
									VariableName: "environment",
								},
							},
						},
					},
				},
			},
			"key3": {
				Literal: &ScalarValue{IntValue: &expectedIntVal},
			},
		},
	})
}

func (s *MappingNodeTestSuite) Test_parse_items_val_yaml(c *C) {
	targetMappingNode := &MappingNode{}
	err := yaml.Unmarshal(s.specParseFixtures["itemsValYAML"], targetMappingNode)
	c.Assert(err, IsNil)
	assertItemsNodeYAML(c, targetMappingNode)
}

func (s *MappingNodeTestSuite) Test_parse_items_val_json(c *C) {
	targetMappingNode := &MappingNode{}
	err := json.Unmarshal(s.specParseFixtures["itemsValJSON"], targetMappingNode)
	c.Assert(err, IsNil)
	assertItemsNodeJSON(c, targetMappingNode)
}

func assertItemsNodeYAML(c *C, actual *MappingNode) {
	expectedIntVal := 45172131
	expectedStrVal := "value1"
	expectedStrSubPrefix := "value with sub "
	c.Assert(actual, DeepEquals, &MappingNode{
		Items: []*MappingNode{
			{
				Literal: &ScalarValue{
					StringValue: &expectedStrVal,
					SourceMeta:  &source.Meta{Line: 2, Column: 11},
				},
			},
			{
				StringWithSubstitutions: &substitutions.StringOrSubstitutions{
					Values: []*substitutions.StringOrSubstitution{
						{StringValue: &expectedStrSubPrefix},
						{
							SubstitutionValue: &substitutions.Substitution{
								Variable: &substitutions.SubstitutionVariable{
									VariableName: "environment",
								},
							},
						},
					},
				},
			},
			{
				Literal: &ScalarValue{
					IntValue:   &expectedIntVal,
					SourceMeta: &source.Meta{Line: 4, Column: 11},
				},
			},
		},
	})
}

func assertItemsNodeJSON(c *C, actual *MappingNode) {
	expectedIntVal := 45172131
	expectedStrVal := "value1"
	expectedStrSubPrefix := "value with sub "
	c.Assert(actual, DeepEquals, &MappingNode{
		Items: []*MappingNode{
			{
				Literal: &ScalarValue{
					StringValue: &expectedStrVal,
				},
			},
			{
				StringWithSubstitutions: &substitutions.StringOrSubstitutions{
					Values: []*substitutions.StringOrSubstitution{
						{StringValue: &expectedStrSubPrefix},
						{
							SubstitutionValue: &substitutions.Substitution{
								Variable: &substitutions.SubstitutionVariable{
									VariableName: "environment",
								},
							},
						},
					},
				},
			},
			{
				Literal: &ScalarValue{
					IntValue: &expectedIntVal,
				},
			},
		},
	})
}

func (s *MappingNodeTestSuite) Test_parse_nested_val_yaml(c *C) {
	targetMappingNode := &MappingNode{}
	err := yaml.Unmarshal(s.specParseFixtures["nestedValYAML"], targetMappingNode)
	c.Assert(err, IsNil)
	assertNestedNodeYAML(c, targetMappingNode)
}

func (s *MappingNodeTestSuite) Test_parse_nested_val_json(c *C) {
	targetMappingNode := &MappingNode{}
	err := json.Unmarshal(s.specParseFixtures["nestedValJSON"], targetMappingNode)
	c.Assert(err, IsNil)
	assertNestedNodeJSON(c, targetMappingNode)
}

func assertNestedNodeJSON(c *C, actual *MappingNode) {
	expectedIntVal := 931721304
	expectedStrVal1 := "value10"
	expectedStrVal2 := "value11"
	expectedStrVal3 := "value12"
	expectedStrSubPrefix := "value13 with sub "
	c.Assert(actual, DeepEquals, &MappingNode{
		Fields: map[string]*MappingNode{
			"key1": {
				Literal: &ScalarValue{StringValue: &expectedStrVal1},
			},
			"key2": {
				Fields: map[string]*MappingNode{
					"key3": {
						Literal: &ScalarValue{StringValue: &expectedStrVal2},
					},
				},
			},
			"key4": {
				Items: []*MappingNode{
					{
						Literal: &ScalarValue{StringValue: &expectedStrVal3},
					},
					{
						StringWithSubstitutions: &substitutions.StringOrSubstitutions{
							Values: []*substitutions.StringOrSubstitution{
								{StringValue: &expectedStrSubPrefix},
								{
									SubstitutionValue: &substitutions.Substitution{
										Variable: &substitutions.SubstitutionVariable{
											VariableName: "environment",
										},
									},
								},
							},
						},
					},
				},
			},
			"key5": {
				Literal: &ScalarValue{IntValue: &expectedIntVal},
			},
		},
	})
}

func assertNestedNodeYAML(c *C, actual *MappingNode) {
	expectedIntVal := 931721304
	expectedStrVal1 := "value10"
	expectedStrVal2 := "value11"
	expectedStrVal3 := "value12"
	expectedStrSubPrefix := "value13 with sub "
	c.Assert(actual, DeepEquals, &MappingNode{
		Fields: map[string]*MappingNode{
			"key1": {
				Literal: &ScalarValue{
					StringValue: &expectedStrVal1,
					SourceMeta:  &source.Meta{Line: 2, Column: 17},
				},
			},
			"key2": {
				Fields: map[string]*MappingNode{
					"key3": {
						Literal: &ScalarValue{
							StringValue: &expectedStrVal2,
							SourceMeta:  &source.Meta{Line: 4, Column: 19},
						},
					},
				},
			},
			"key4": {
				Items: []*MappingNode{
					{
						Literal: &ScalarValue{
							StringValue: &expectedStrVal3,
							SourceMeta:  &source.Meta{Line: 6, Column: 14},
						},
					},
					{
						StringWithSubstitutions: &substitutions.StringOrSubstitutions{
							Values: []*substitutions.StringOrSubstitution{
								{StringValue: &expectedStrSubPrefix},
								{
									SubstitutionValue: &substitutions.Substitution{
										Variable: &substitutions.SubstitutionVariable{
											VariableName: "environment",
										},
									},
								},
							},
						},
					},
				},
			},
			"key5": {
				Literal: &ScalarValue{
					IntValue:   &expectedIntVal,
					SourceMeta: &source.Meta{Line: 8, Column: 17},
				},
			},
		},
	})
}

func (s *MappingNodeTestSuite) Test_fails_to_parse_invalid_value(c *C) {
	targetMappingNode := &MappingNode{}
	err := yaml.Unmarshal(s.specParseFixtures["failInvalidValue"], targetMappingNode)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "a blueprint mapping node must be a valid scalar, mapping or sequence")
}

func (s *MappingNodeTestSuite) Test_serialise_string_val_yaml(c *C) {
	actual, err := yaml.Marshal(s.specSerialiseFixtures["stringValYAML"])
	c.Assert(err, IsNil)
	c.Assert(string(actual), Equals, "Test string value\n")
}

func (s *MappingNodeTestSuite) Test_serialise_string_val_json(c *C) {
	actual, err := json.Marshal(s.specSerialiseFixtures["stringValJSON"])
	c.Assert(err, IsNil)
	c.Assert(string(actual), Equals, "\"Test string value\"")
}

func (s *MappingNodeTestSuite) Test_serialise_string_with_subs_yaml(c *C) {
	actual, err := yaml.Marshal(s.specSerialiseFixtures["stringWithSubsYAML"])
	c.Assert(err, IsNil)
	c.Assert(string(actual), Equals, "Test string value for ${variables.environment}\n")
}

func (s *MappingNodeTestSuite) Test_serialise_string_with_subs_json(c *C) {
	actual, err := json.Marshal(s.specSerialiseFixtures["stringWithSubsJSON"])
	c.Assert(err, IsNil)
	c.Assert(string(actual), Equals, "\"Test string value for ${variables.environment}\"")
}

func (s *MappingNodeTestSuite) Test_serialise_int_val_yaml(c *C) {
	actual, err := yaml.Marshal(s.specSerialiseFixtures["intVal"])
	c.Assert(err, IsNil)
	c.Assert(string(actual), Equals, "45172131\n")
}

func (s *MappingNodeTestSuite) Test_serialise_int_val_json(c *C) {
	actual, err := json.Marshal(s.specSerialiseFixtures["intVal"])
	c.Assert(err, IsNil)
	c.Assert(string(actual), Equals, "45172131")
}

func (s *MappingNodeTestSuite) Test_serialise_fields_val_yaml(c *C) {
	actual, err := yaml.Marshal(s.specSerialiseFixtures["fieldsValYAML"])
	c.Assert(err, IsNil)
	c.Assert(string(actual), Equals, "key1: Test string value\nkey2: Test string value for ${variables.environment}\nkey3: 45172131\n")
}

func (s *MappingNodeTestSuite) Test_serialise_fields_val_json(c *C) {
	actual, err := json.Marshal(s.specSerialiseFixtures["fieldsValJSON"])
	c.Assert(err, IsNil)
	c.Assert(string(actual), Equals, "{\"key1\":\"Test string value\",\"key2\":\"Test string value for ${variables.environment}\",\"key3\":45172131}")
}

func (s *MappingNodeTestSuite) Test_serialise_items_val_yaml(c *C) {
	actual, err := yaml.Marshal(s.specSerialiseFixtures["itemsValYAML"])
	c.Assert(err, IsNil)
	c.Assert(string(actual), Equals, "- Test string value\n- Test string value for ${variables.environment}\n- 45172131\n")
}

func (s *MappingNodeTestSuite) Test_serialise_items_val_json(c *C) {
	actual, err := json.Marshal(s.specSerialiseFixtures["itemsValJSON"])
	c.Assert(err, IsNil)
	c.Assert(string(actual), Equals, "[\"Test string value\",\"Test string value for ${variables.environment}\",45172131]")
}

func (s *MappingNodeTestSuite) Test_fails_to_serialise_invalid_mapping_node_yaml(c *C) {
	_, err := yaml.Marshal(s.specSerialiseFixtures["failInvalidYAML"])
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "a blueprint mapping node must have a valid value set")
}

func (s *MappingNodeTestSuite) Test_fails_to_serialise_invalid_mapping_node_json(c *C) {
	_, err := json.Marshal(s.specSerialiseFixtures["failInvalidJSON"])
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "json: error calling MarshalJSON for type *core.MappingNode: a blueprint mapping node must have a valid value set")
}

func (s *MappingNodeTestSuite) prepareParseInputFixtures() {
	s.specParseFixtures = map[string][]byte{
		"stringValYAML":      []byte(testStringValue),
		"stringValJSON":      []byte("\"Test string value\""),
		"stringWithSubsYAML": []byte("Test string value for ${variables.environment}"),
		"stringWithSubsJSON": []byte("\"Test string value for ${variables.environment}\""),
		// We only need one example of a non-string scalar value as scalar value
		// parsing is tested in the ScalarValue test suite.
		"intVal": []byte("45172131"),
		"fieldsValYAML": []byte(`
        key1: "value1"
        key2: "value with sub ${variables.environment}"
        key3: 45172131`),
		"fieldsValJSON": []byte(`{
			"key1": "value1",
			"key2": "value with sub ${variables.environment}",
			"key3": 45172131
			}`),
		"itemsValYAML": []byte(`
        - "value1"
        - "value with sub ${variables.environment}"
        - 45172131`),
		"itemsValJSON": []byte(`[
			"value1",
			"value with sub ${variables.environment}",
			45172131
			]`),
		"nestedValYAML": []byte(`
          key1: "value10"
          key2:
            key3: "value11"
          key4:
           - "value12"
           - "value13 with sub ${variables.environment}"
          key5: 931721304`),
		"nestedValJSON": []byte(`{
			"key1": "value10",
			"key2": {
				"key3": "value11"
			},
			"key4": [
				"value12",
				"value13 with sub ${variables.environment}"
			],
			"key5": 931721304
			}`),
		// YAML anchors and aliases do not represent valid values for a mapping node.
		"failInvalidValue": []byte(`
        - &unsupportedAlias
        - *unsupportedAlias`),
	}
}

func (s *MappingNodeTestSuite) prepareExpectedFixtures() {
	expectedStrVal := testStringValue
	expectedSubStrVal := "Test string value for "
	expectedMappingNodeWithSubs := &MappingNode{
		StringWithSubstitutions: &substitutions.StringOrSubstitutions{
			Values: []*substitutions.StringOrSubstitution{
				{StringValue: &expectedSubStrVal},
				{
					SubstitutionValue: &substitutions.Substitution{
						Variable: &substitutions.SubstitutionVariable{
							VariableName: "environment",
						},
					},
				},
			},
		},
	}
	expectedIntVal := 45172131
	expectedFields := map[string]*MappingNode{
		"key1": {
			Literal: &ScalarValue{StringValue: &expectedStrVal},
		},
		"key2": {
			StringWithSubstitutions: &substitutions.StringOrSubstitutions{
				Values: []*substitutions.StringOrSubstitution{
					{StringValue: &expectedSubStrVal},
					{
						SubstitutionValue: &substitutions.Substitution{
							Variable: &substitutions.SubstitutionVariable{
								VariableName: "environment",
							},
						},
					},
				},
			},
		},
		"key3": {
			Literal: &ScalarValue{IntValue: &expectedIntVal},
		},
	}
	expectedItems := []*MappingNode{
		{
			Literal: &ScalarValue{StringValue: &expectedStrVal},
		},
		{
			StringWithSubstitutions: &substitutions.StringOrSubstitutions{
				Values: []*substitutions.StringOrSubstitution{
					{StringValue: &expectedSubStrVal},
					{
						SubstitutionValue: &substitutions.Substitution{
							Variable: &substitutions.SubstitutionVariable{
								VariableName: "environment",
							},
						},
					},
				},
			},
		},
		{
			Literal: &ScalarValue{IntValue: &expectedIntVal},
		},
	}

	s.specSerialiseFixtures = map[string]*MappingNode{
		"stringValYAML": {
			Literal: &ScalarValue{StringValue: &expectedStrVal},
		},
		"stringValJSON": {
			Literal: &ScalarValue{StringValue: &expectedStrVal},
		},
		"stringWithSubsYAML": expectedMappingNodeWithSubs,
		"stringWithSubsJSON": expectedMappingNodeWithSubs,
		"intVal": {
			Literal: &ScalarValue{IntValue: &expectedIntVal},
		},
		"fieldsValYAML": {
			Fields: expectedFields,
		},
		"fieldsValJSON": {
			Fields: expectedFields,
		},
		"itemsValYAML": {
			Items: expectedItems,
		},
		"itemsValJSON": {
			Items: expectedItems,
		},
		// Empty mapping node is invalid.
		"failInvalidYAML": {},
		"failInvalidJSON": {},
	}
}

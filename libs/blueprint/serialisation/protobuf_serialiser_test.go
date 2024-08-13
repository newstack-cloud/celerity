package serialisation

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v3"
)

func Test(t *testing.T) {
	TestingT(t)
}

type fixture struct {
	filePath  string
	blueprint *schema.Blueprint
}

type ProtobufSerialiserTestSuite struct {
	specFixtures map[string]fixture
}

var _ = Suite(&ProtobufSerialiserTestSuite{})

func (s *ProtobufSerialiserTestSuite) SetUpSuite(c *C) {
	s.specFixtures = make(map[string]fixture)
	fixturesToLoad := map[string]string{
		"fixture1": "__testdata/blueprint-required-only.yml",
		"fixture2": "__testdata/blueprint-full.yml",
	}

	for name, filePath := range fixturesToLoad {
		specBytes, err := os.ReadFile(filePath)
		if err != nil {
			c.Error(err)
			c.FailNow()
		}
		blueprint := &schema.Blueprint{}
		err = yaml.Unmarshal(specBytes, blueprint)
		if err != nil {
			c.Error(err)
			c.FailNow()
		}

		s.specFixtures[name] = fixture{
			filePath:  filePath,
			blueprint: blueprint,
		}
	}
}

func (s *ProtobufSerialiserTestSuite) Test_marshals_and_unmarshals_blueprint_fixture_1(c *C) {
	serialiser := NewProtobufSerialiser()
	serialised, err := serialiser.Marshal(s.specFixtures["fixture1"].blueprint)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	unmarshalled, err := serialiser.Unmarshal(serialised)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	// Compare serialised JSON versions as not all structs and maps are equal at runtime.
	// (nil maps are not equal to empty initialised maps, for example.)
	serialisedActualJSON, err := json.Marshal(unmarshalled)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	serialisedExpectedJSON, err := json.Marshal(s.specFixtures["fixture1"].blueprint)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(string(serialisedActualJSON), Equals, string(serialisedExpectedJSON))
}

func (s *ProtobufSerialiserTestSuite) Test_marshals_and_unmarshals_blueprint_fixture_2(c *C) {
	serialiser := NewProtobufSerialiser()
	serialised, err := serialiser.Marshal(s.specFixtures["fixture2"].blueprint)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	unmarshalled, err := serialiser.Unmarshal(serialised)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	// Compare serialised JSON versions as not all structs and maps are equal at runtime.
	// (nil maps are not equal to empty initialised maps, for example.)
	serialisedActualJSON, err := json.Marshal(unmarshalled)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	serialisedExpectedJSON, err := json.Marshal(s.specFixtures["fixture2"].blueprint)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(string(serialisedActualJSON), Equals, string(serialisedExpectedJSON))
}

func (s *ProtobufSerialiserTestSuite) Test_fails_to_marshal_blueprint_fixture_with_missing_scalar(c *C) {
	serialiser := NewProtobufSerialiser()
	_, err := serialiser.Marshal(fixtureMissingScalarField)
	if err == nil {
		c.Error("expected error, got nil")
		c.FailNow()
	}

	if err.Error() != "expanded blueprint serialise error: missing scalar value" {
		c.Errorf("unexpected error message: %s", err.Error())
	}
}

func (s *ProtobufSerialiserTestSuite) Test_fails_to_marshal_blueprint_fixture_with_missing_string_or_substitution(c *C) {
	serialiser := NewProtobufSerialiser()
	_, err := serialiser.Marshal(fixtureMissingStringOrSubstitutionField)
	if err == nil {
		c.Error("expected error, got nil")
		c.FailNow()
	}

	if err.Error() != "expanded blueprint serialise error: missing string or substitution value" {
		c.Errorf("unexpected error message: %s", err.Error())
	}
}

func (s *ProtobufSerialiserTestSuite) Test_fails_to_marshal_blueprint_fixture_with_missing_substitution(c *C) {
	serialiser := NewProtobufSerialiser()
	_, err := serialiser.Marshal(fixtureMissingSubstitutionField)
	if err == nil {
		c.Error("expected error, got nil")
		c.FailNow()
	}

	if err.Error() != "expanded blueprint serialise error: missing substitution value" {
		c.Errorf("unexpected error message: %s", err.Error())
	}
}

func (s *ProtobufSerialiserTestSuite) Test_fails_to_marshal_blueprint_fixture_with_missing_substitution_path_item(c *C) {
	serialiser := NewProtobufSerialiser()
	_, err := serialiser.Marshal(fixtureMissingSubstitutionPathItemField)
	if err == nil {
		c.Error("expected error, got nil")
		c.FailNow()
	}

	if err.Error() != "expanded blueprint serialise error: missing substitution path item value" {
		c.Errorf("unexpected error message: %s", err.Error())
	}
}

var testRuntime = "go1.x"
var testTracingEnabled = true

// Error fixtures are initialised in-memory as JSON and YAML versions can not get to these
// error states, these errors reveal code errors and not user input errors.

var fixtureMissingScalarField = &schema.Blueprint{
	Version: "2021-12-18",
	Resources: &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"orderApi": {
				Type: "celerity/api",
				Spec: &core.MappingNode{
					Fields: map[string]*core.MappingNode{
						"environment": {
							Fields: map[string]*core.MappingNode{
								"variables": {
									Fields: map[string]*core.MappingNode{
										"DYNAMODB_TABLE": {
											StringWithSubstitutions: &substitutions.StringOrSubstitutions{
												Values: []*substitutions.StringOrSubstitution{
													{
														SubstitutionValue: &substitutions.Substitution{
															Variable: &substitutions.SubstitutionVariable{
																VariableName: "dynamoDBTable",
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
						"runtime": {
							Literal: &core.ScalarValue{
								// A scalar value is missing here.
							},
						},
						"tracingEnabled": {
							Literal: &core.ScalarValue{
								BoolValue: &testTracingEnabled,
							},
						},
					},
				},
			},
		},
	},
}

var fixtureMissingStringOrSubstitutionField = &schema.Blueprint{
	Version: "2021-12-18",
	Resources: &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"orderApi": {
				Type: "celerity/api",
				Spec: &core.MappingNode{
					Fields: map[string]*core.MappingNode{
						"environment": {
							Fields: map[string]*core.MappingNode{
								"variables": {
									Fields: map[string]*core.MappingNode{
										"DYNAMODB_TABLE": {
											StringWithSubstitutions: &substitutions.StringOrSubstitutions{
												Values: []*substitutions.StringOrSubstitution{
													{
														// A string or substitution value is missing here.
													},
												},
											},
										},
									},
								},
							},
						},
						"runtime": {
							Literal: &core.ScalarValue{
								StringValue: &testRuntime,
							},
						},
						"tracingEnabled": {
							Literal: &core.ScalarValue{
								BoolValue: &testTracingEnabled,
							},
						},
					},
				},
			},
		},
	},
}

var fixtureMissingSubstitutionField = &schema.Blueprint{
	Version: "2021-12-18",
	Resources: &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"orderApi": {
				Type: "celerity/api",
				Spec: &core.MappingNode{
					Fields: map[string]*core.MappingNode{
						"environment": {
							Fields: map[string]*core.MappingNode{
								"variables": {
									Fields: map[string]*core.MappingNode{
										"DYNAMODB_TABLE": {
											StringWithSubstitutions: &substitutions.StringOrSubstitutions{
												Values: []*substitutions.StringOrSubstitution{
													{
														SubstitutionValue: &substitutions.Substitution{
															// A substitution value is missing here.
														},
													},
												},
											},
										},
									},
								},
							},
						},
						"runtime": {
							Literal: &core.ScalarValue{
								StringValue: &testRuntime,
							},
						},
						"tracingEnabled": {
							Literal: &core.ScalarValue{
								BoolValue: &testTracingEnabled,
							},
						},
					},
				},
			},
		},
	},
}

var fixtureMissingSubstitutionPathItemField = &schema.Blueprint{
	Version: "2021-12-18",
	Resources: &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"orderApi": {
				Type: "celerity/api",
				Spec: &core.MappingNode{
					Fields: map[string]*core.MappingNode{
						"environment": {
							Fields: map[string]*core.MappingNode{
								"variables": {
									Fields: map[string]*core.MappingNode{
										"DYNAMODB_TABLE": {
											StringWithSubstitutions: &substitutions.StringOrSubstitutions{
												Values: []*substitutions.StringOrSubstitution{
													{
														SubstitutionValue: &substitutions.Substitution{
															ResourceProperty: &substitutions.SubstitutionResourceProperty{
																ResourceName: "dynamoDBTable",
																Path: []*substitutions.SubstitutionPathItem{
																	{
																		// A substitution path item is missing here.
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
						"runtime": {
							Literal: &core.ScalarValue{
								StringValue: &testRuntime,
							},
						},
						"tracingEnabled": {
							Literal: &core.ScalarValue{
								BoolValue: &testTracingEnabled,
							},
						},
					},
				},
			},
		},
	},
}

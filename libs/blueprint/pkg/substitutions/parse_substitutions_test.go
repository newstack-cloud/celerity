package substitutions

import (
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) {
	TestingT(t)
}

type ParseSubstitutionsTestSuite struct{}

var _ = Suite(&ParseSubstitutionsTestSuite{})

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_multiple_substitutions(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		"${replace(datasources.host.domain, \"${}\", \"\")}/${variables.version}/app",
	)
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 4)

	arg2 := "${}"
	arg3 := ""
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			Function: &SubstitutionFunction{
				FunctionName: "replace",
				Arguments: []*Substitution{
					{
						DataSourceProperty: &SubstitutionDataSourceProperty{
							DataSourceName: "host",
							FieldName:      "domain",
						},
					},
					{
						StringValue: &arg2,
					},
					{
						StringValue: &arg3,
					},
				},
			},
		},
	})

	pathSeparator := "/"
	c.Assert(parsed[1], DeepEquals, &StringOrSubstitution{
		StringValue: &pathSeparator,
	})

	c.Assert(parsed[2], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			Variable: &SubstitutionVariable{
				VariableName: "version",
			},
		},
	})

	pathSuffix := "/app"
	c.Assert(parsed[3], DeepEquals, &StringOrSubstitution{
		StringValue: &pathSuffix,
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_a_data_source_ref_sub_1(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${datasources["coreInfra.v1"]["topic.v2"][0]}`,
	)
	index := int64(0)
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 1)
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			DataSourceProperty: &SubstitutionDataSourceProperty{
				DataSourceName:    "coreInfra.v1",
				FieldName:         "topic.v2",
				PrimitiveArrIndex: &index,
			},
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_a_data_source_ref_sub_2(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		"${datasources.coreInfra1.topics[1]}",
	)
	index := int64(1)
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 1)
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			DataSourceProperty: &SubstitutionDataSourceProperty{
				DataSourceName:    "coreInfra1",
				FieldName:         "topics",
				PrimitiveArrIndex: &index,
			},
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_a_child_ref_sub_1(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${children["core-infrastructure.v1"].cacheNodes[].host}`,
	)
	index := int64(0)
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 1)
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			Child: &SubstitutionChild{
				ChildName: "core-infrastructure.v1",
				Path: []*SubstitutionPathItem{
					{
						FieldName: "cacheNodes",
					},
					{
						PrimitiveArrIndex: &index,
					},
					{
						FieldName: "host",
					},
				},
			},
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_a_resource_ref_sub_1(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${resources.saveOrderFunction.metadata.annotations["annotationKey.v1"]}`,
	)
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 1)
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			ResourceProperty: &SubstitutionResourceProperty{
				ResourceName: "saveOrderFunction",
				Path: []*SubstitutionPathItem{
					{
						FieldName: "metadata",
					},
					{
						FieldName: "annotations",
					},
					{
						FieldName: "annotationKey.v1",
					},
				},
			},
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_a_resource_ref_sub_2(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${resources["save-order-function.v1"].state.functionArn}`,
	)
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 1)
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			ResourceProperty: &SubstitutionResourceProperty{
				ResourceName: "save-order-function.v1",
				Path: []*SubstitutionPathItem{
					{
						FieldName: "state",
					},
					{
						FieldName: "functionArn",
					},
				},
			},
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_a_resource_ref_sub_3(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${saveOrderFunction}`,
	)
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 1)
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			ResourceProperty: &SubstitutionResourceProperty{
				ResourceName: "saveOrderFunction",
				Path:         []*SubstitutionPathItem{},
			},
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_sub_string_with_a_string_literal(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${  "This is a \"string\" literal"    }`,
	)
	expectedStrVal := "This is a \"string\" literal"
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 1)
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			StringValue: &expectedStrVal,
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_sub_string_with_a_func_call_1(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${  substr(trim("This is a \"string\" literal"), 0, 3)    }`,
	)
	trimArg := "This is a \"string\" literal"
	arg2 := int64(0)
	arg3 := int64(3)
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 1)
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			Function: &SubstitutionFunction{
				FunctionName: "substr",
				Arguments: []*Substitution{
					{
						Function: &SubstitutionFunction{
							FunctionName: "trim",
							Arguments: []*Substitution{
								{
									StringValue: &trimArg,
								},
							},
						},
					},
					{
						IntValue: &arg2,
					},
					{
						IntValue: &arg3,
					},
				},
			},
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_sub_string_with_a_func_call_2(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${  trim("This is a \"string\" literal", true)    }`,
	)
	arg1 := "This is a \"string\" literal"
	arg2 := true
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 1)
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			Function: &SubstitutionFunction{
				FunctionName: "trim",
				Arguments: []*Substitution{
					{
						StringValue: &arg1,
					},
					{
						BoolValue: &arg2,
					},
				},
			},
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_sub_string_with_a_func_call_3(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${  format(25.40932102)   }`,
	)
	arg := 25.40932102
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 1)
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			Function: &SubstitutionFunction{
				FunctionName: "format",
				Arguments: []*Substitution{
					{
						FloatValue: &arg,
					},
				},
			},
		},
	})
}

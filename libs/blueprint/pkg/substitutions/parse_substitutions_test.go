package substitutions

import (
	"testing"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/source"
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
		// Emulate the inner substitution starting on line 200, column 100,
		// outer column is 98.
		// Source meta values of substitution components are offset from the start
		// of the input string.
		&source.Meta{
			Line:   200,
			Column: 100,
		},
		true,
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
							SourceMeta: &source.Meta{
								Line:   200,
								Column: 110,
							},
						},
						SourceMeta: &source.Meta{
							Line:   200,
							Column: 110,
						},
					},
					{
						StringValue: &arg2,
						SourceMeta: &source.Meta{
							Line:   200,
							Column: 135,
						},
					},
					{
						StringValue: &arg3,
						SourceMeta: &source.Meta{
							Line:   200,
							Column: 142,
						},
					},
				},
				SourceMeta: &source.Meta{
					Line:   200,
					Column: 102,
				},
			},
			SourceMeta: &source.Meta{
				Line:   200,
				Column: 102,
			},
		},
		SourceMeta: &source.Meta{
			Line:   200,
			Column: 100,
		},
	})

	pathSeparator := "/"
	c.Assert(parsed[1], DeepEquals, &StringOrSubstitution{
		StringValue: &pathSeparator,
		SourceMeta: &source.Meta{
			Line:   200,
			Column: 146,
		},
	})

	c.Assert(parsed[2], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			Variable: &SubstitutionVariable{
				VariableName: "version",
				SourceMeta: &source.Meta{
					Line:   200,
					Column: 149,
				},
			},
			SourceMeta: &source.Meta{
				Line:   200,
				Column: 149,
			},
		},
		SourceMeta: &source.Meta{
			Line:   200,
			Column: 147,
		},
	})

	pathSuffix := "/app"
	c.Assert(parsed[3], DeepEquals, &StringOrSubstitution{
		StringValue: &pathSuffix,
		SourceMeta: &source.Meta{
			Line:   200,
			Column: 167,
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_a_data_source_ref_sub_1(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${datasources["coreInfra.v1"]["topic.v2"][0]}`,
		nil,
		true,
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
				SourceMeta: &source.Meta{
					Line:   1,
					Column: 3,
				},
			},
			SourceMeta: &source.Meta{
				Line:   1,
				Column: 3,
			},
		},
		SourceMeta: &source.Meta{
			Line:   1,
			Column: 3,
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_a_data_source_ref_sub_2(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		"${datasources.coreInfra1.topics[1]}",
		nil,
		true,
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
				SourceMeta: &source.Meta{
					Line:   1,
					Column: 3,
				},
			},
			SourceMeta: &source.Meta{
				Line:   1,
				Column: 3,
			},
		},
		SourceMeta: &source.Meta{
			Line:   1,
			Column: 3,
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_a_child_ref_sub_1(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${children["core-infrastructure.v1"].cacheNodes[].host}`,
		nil,
		true,
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
				SourceMeta: &source.Meta{
					Line:   1,
					Column: 3,
				},
			},
			SourceMeta: &source.Meta{
				Line:   1,
				Column: 3,
			},
		},
		SourceMeta: &source.Meta{
			Line:   1,
			Column: 3,
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_a_resource_ref_sub_1(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${resources.saveOrderFunction.metadata.annotations["annotationKey.v1"]}`,
		nil,
		true,
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
				SourceMeta: &source.Meta{
					Line:   1,
					Column: 3,
				},
			},
			SourceMeta: &source.Meta{
				Line:   1,
				Column: 3,
			},
		},
		SourceMeta: &source.Meta{
			Line:   1,
			Column: 3,
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_a_resource_ref_sub_2(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${resources["save-order-function.v1"].state.functionArn}`,
		nil,
		true,
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
				SourceMeta: &source.Meta{
					Line:   1,
					Column: 3,
				},
			},
			SourceMeta: &source.Meta{
				Line:   1,
				Column: 3,
			},
		},
		SourceMeta: &source.Meta{
			Line:   1,
			Column: 3,
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_a_resource_ref_sub_3(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${saveOrderFunction}`,
		nil,
		true,
	)
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 1)
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			ResourceProperty: &SubstitutionResourceProperty{
				ResourceName: "saveOrderFunction",
				Path:         []*SubstitutionPathItem{},
				SourceMeta: &source.Meta{
					Line:   1,
					Column: 3,
				},
			},
			SourceMeta: &source.Meta{
				Line:   1,
				Column: 3,
			},
		},
		SourceMeta: &source.Meta{
			Line:   1,
			Column: 3,
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_sub_string_with_a_string_literal(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${  "This is a \"string\" literal"    }`,
		nil,
		true,
	)
	expectedStrVal := "This is a \"string\" literal"
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 1)
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			StringValue: &expectedStrVal,
			SourceMeta: &source.Meta{
				Line:   1,
				Column: 5,
			},
		},
		SourceMeta: &source.Meta{
			Line:   1,
			Column: 3,
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_sub_string_with_a_func_call_1(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${  substr(trim("This is a \"string\" literal"), 0, 3)    }`,
		nil,
		true,
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
									SourceMeta: &source.Meta{
										Line:   1,
										Column: 17,
									},
								},
							},
							SourceMeta: &source.Meta{
								Line:   1,
								Column: 12,
							},
						},
						SourceMeta: &source.Meta{
							Line:   1,
							Column: 12,
						},
					},
					{
						IntValue: &arg2,
						SourceMeta: &source.Meta{
							Line:   1,
							Column: 50,
						},
					},
					{
						IntValue: &arg3,
						SourceMeta: &source.Meta{
							Line:   1,
							Column: 53,
						},
					},
				},
				SourceMeta: &source.Meta{
					Line:   1,
					Column: 5,
				},
			},
			SourceMeta: &source.Meta{
				Line:   1,
				Column: 5,
			},
		},
		SourceMeta: &source.Meta{
			Line:   1,
			Column: 3,
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_sub_string_with_a_func_call_2(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${  trim("This is a \"string\" literal", true)    }`,
		nil,
		true,
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
						SourceMeta: &source.Meta{
							Line:   1,
							Column: 10,
						},
					},
					{
						BoolValue: &arg2,
						SourceMeta: &source.Meta{
							Line:   1,
							Column: 42,
						},
					},
				},
				SourceMeta: &source.Meta{
					Line:   1,
					Column: 5,
				},
			},
			SourceMeta: &source.Meta{
				Line:   1,
				Column: 5,
			},
		},
		SourceMeta: &source.Meta{
			Line:   1,
			Column: 3,
		},
	})
}

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_sub_string_with_a_func_call_3(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${  format(25.40932102)   }`,
		// Emulate this substitution starting on line 100, column 50.
		// Source meta values of substitution components are offset from the start
		// of the input string.
		&source.Meta{
			Line:   100,
			Column: 50,
		},
		true,
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
						SourceMeta: &source.Meta{
							Line:   100,
							Column: 61,
						},
					},
				},
				SourceMeta: &source.Meta{
					Line:   100,
					Column: 54,
				},
			},
			SourceMeta: &source.Meta{
				Line:   100,
				Column: 54,
			},
		},
		SourceMeta: &source.Meta{
			Line:   100,
			Column: 50,
		},
	})
}

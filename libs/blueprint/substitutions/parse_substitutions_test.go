package substitutions

import (
	"testing"

	"github.com/two-hundred/celerity/libs/blueprint/errors"
	"github.com/two-hundred/celerity/libs/blueprint/source"
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
		false,
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
		true,  // outputLineInfo
		false, // ignoreParentColumn
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
		false,
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
		false,
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
		false,
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
		false,
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
		false,
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

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_a_value_ref_sub_1(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${values.s3Bucket.info["objectConfig"][3]}`,
		nil,
		true,
		false,
	)
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 1)
	arrIndex := int64(3)
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			ValueReference: &SubstitutionValueReference{
				ValueName: "s3Bucket",
				Path: []*SubstitutionPathItem{
					{
						FieldName: "info",
					},
					{
						FieldName: "objectConfig",
					},
					{
						PrimitiveArrIndex: &arrIndex,
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

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_a_value_ref_sub_2(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${values.googleCloudBuckets[1].name}`,
		nil,
		true,
		false,
	)
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 1)
	arrIndex := int64(1)
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			ValueReference: &SubstitutionValueReference{
				ValueName: "googleCloudBuckets",
				Path: []*SubstitutionPathItem{
					{
						PrimitiveArrIndex: &arrIndex,
					},
					{
						FieldName: "name",
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

func (s *ParseSubstitutionsTestSuite) Test_correctly_parses_a_string_with_a_value_ref_sub_3(c *C) {
	parsed, err := ParseSubstitutionValues(
		"",
		`${values.queueUrl}`,
		nil,
		true,
		false,
	)
	c.Assert(err, IsNil)
	c.Assert(len(parsed), Equals, 1)
	c.Assert(parsed[0], DeepEquals, &StringOrSubstitution{
		SubstitutionValue: &Substitution{
			ValueReference: &SubstitutionValueReference{
				ValueName: "queueUrl",
				Path:      []*SubstitutionPathItem{},
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
		false,
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
		false,
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
		false,
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
		false,
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

func (s *ParseSubstitutionsTestSuite) Test_fails_to_parse_susbstitution_reporting_correct_position(c *C) {
	_, err := ParseSubstitutionValues(
		"",
		// hex numbers are not supported in the substitution language.
		`${  format(0x23)   }`,
		// Emulate this substitution starting on line 100, column 50.
		// Source meta values of substitution components are offset from the start
		// of the input string.
		&source.Meta{
			Line:   100,
			Column: 50,
		},
		true,
		false,
	)
	c.Assert(err, NotNil)

	loadErr, isLoadErr := err.(*errors.LoadError)
	c.Assert(isLoadErr, Equals, true)
	// Top-level error corresponds to the outer start point
	// of the substitution (the location of the "${").
	c.Assert(*loadErr.Line, Equals, 100)
	c.Assert(*loadErr.Column, Equals, 50)

	parseErrs, isParseErrs := loadErr.ChildErrors[0].(*ParseErrors)
	c.Assert(isParseErrs, Equals, true)
	c.Assert(parseErrs.ChildErrors, HasLen, 1)

	parseErr, isParseErr := parseErrs.ChildErrors[0].(*ParseError)
	c.Assert(isParseErr, Equals, true)
	// The parse error corresponds to the "x" in "0x23"
	// which is not expected after the "0".
	c.Assert(parseErr.Line, Equals, 100)
	c.Assert(parseErr.Column, Equals, 62)
	c.Assert(
		parseErr.Error(),
		Equals,
		"parse error at column 62 with token type identifier: "+
			"expected a comma after function argument 0",
	)
}

func (s *ParseSubstitutionsTestSuite) Test_fails_to_parse_susbstitution_reporting_correct_position_for_lex_error(c *C) {
	_, err := ParseSubstitutionValues(
		"",
		// "!" is an unexpected punctuation mark in the substitution language,
		// this should lead to a lex error.
		`${  "start of string literal"!  }`,
		// Emulate this substitution starting on line 150, column 70.
		// Source meta values of substitution components are offset from the start
		// of the input string.
		&source.Meta{
			Line:   150,
			Column: 70,
		},
		true,
		false,
	)
	c.Assert(err, NotNil)

	loadErr, isLoadErr := err.(*errors.LoadError)
	c.Assert(isLoadErr, Equals, true)
	// Top-level error corresponds to the outer start point
	// of the substitution (the location of the "${").
	c.Assert(*loadErr.Line, Equals, 150)
	c.Assert(*loadErr.Column, Equals, 70)

	lexErrs, isLexErrs := loadErr.ChildErrors[0].(*LexErrors)
	c.Assert(isLexErrs, Equals, true)
	c.Assert(lexErrs.ChildErrors, HasLen, 1)

	lexErr, isLexErr := lexErrs.ChildErrors[0].(*LexError)
	c.Assert(isLexErr, Equals, true)
	// The lex error corresponds to the "!" after the string literal.
	c.Assert(lexErr.Line, Equals, 150)
	c.Assert(lexErr.Column, Equals, 99)
	c.Assert(
		lexErr.Error(),
		Equals,
		"lex error at column 99: validation failed due to an unexpected"+
			" character \"!\" having been encountered in a reference substitution",
	)
}

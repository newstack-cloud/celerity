package corefunctions

import (
	"context"
	"fmt"
	"reflect"

	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

// GtFunction provides the implementation of
// a function that checks if a number is greater than another.
type GtFunction struct {
	definition *function.Definition
}

// NewGtFunction creates a new instance of the GtFunction with
// a complete function definition.
func NewGtFunction() provider.Function {
	return &GtFunction{
		definition: &function.Definition{
			Description: "A function that determines whether a number is greater than another number.",
			FormattedDescription: "A function that determines whether a number is greater than another number.\n\n" +
				"**Examples:**\n\n" +
				"```\n${gt(len(datasources.network.subnets), 10)}\n```",
			Parameters: []function.Parameter{
				&function.AnyParameter{
					Label: "a",
					UnionTypes: []function.ValueTypeDefinition{
						&function.ValueTypeDefinitionScalar{
							Type: function.ValueTypeInt64,
						},
						&function.ValueTypeDefinitionScalar{
							Type: function.ValueTypeFloat64,
						},
					},
					Description: "\"a\" in the expression \"a > b\".",
				},
				&function.AnyParameter{
					Label: "b",
					UnionTypes: []function.ValueTypeDefinition{
						&function.ValueTypeDefinitionScalar{
							Type: function.ValueTypeInt64,
						},
						&function.ValueTypeDefinitionScalar{
							Type: function.ValueTypeFloat64,
						},
					},
					Description: "\"b\" in the expression \"a > b\".",
				},
			},
			Return: &function.ScalarReturn{
				Type: &function.ValueTypeDefinitionScalar{
					Label: "boolean",
					Type:  function.ValueTypeBool,
				},
				Description: "True, if the first number is greater than the second number, false otherwise.",
			},
		},
	}
}

func (f *GtFunction) GetDefinition(
	ctx context.Context,
	input *provider.FunctionGetDefinitionInput,
) (*provider.FunctionGetDefinitionOutput, error) {
	return &provider.FunctionGetDefinitionOutput{
		Definition: f.definition,
	}, nil
}

func (f *GtFunction) Call(
	ctx context.Context,
	input *provider.FunctionCallInput,
) (*provider.FunctionCallOutput, error) {
	var lhs interface{}
	var rhs interface{}
	if err := input.Arguments.GetMultipleVars(ctx, &lhs, &rhs); err != nil {
		return nil, err
	}

	lhsInt, isLHSInt := lhs.(int64)
	rhsInt, isRHSInt := rhs.(int64)

	lhsFloat, isLHSFloat := lhs.(float64)
	rhsFloat, isRHSFloat := rhs.(float64)

	if !isLHSInt && !isLHSFloat {
		return nil, function.NewFuncCallError(
			fmt.Sprintf(
				"expected the left-hand side of the comparison to be a number, got %s",
				reflect.TypeOf(lhs),
			),
			function.FuncCallErrorCodeInvalidArgumentType,
			input.CallContext.CallStackSnapshot(),
		)
	}

	if !isRHSInt && !isRHSFloat {
		return nil, function.NewFuncCallError(
			fmt.Sprintf(
				"expected the right-hand side of the comparison to be a number, got %s",
				reflect.TypeOf(rhs),
			),
			function.FuncCallErrorCodeInvalidArgumentType,
			input.CallContext.CallStackSnapshot(),
		)
	}

	if isLHSInt && isRHSInt {
		return &provider.FunctionCallOutput{
			ResponseData: lhsInt > rhsInt,
		}, nil
	}

	if isLHSFloat && isRHSFloat {
		return &provider.FunctionCallOutput{
			ResponseData: lhsFloat > rhsFloat,
		}, nil
	}

	if isLHSInt && isRHSFloat {
		return &provider.FunctionCallOutput{
			ResponseData: float64(lhsInt) > rhsFloat,
		}, nil
	}

	return &provider.FunctionCallOutput{
		ResponseData: lhsFloat > float64(rhsInt),
	}, nil
}

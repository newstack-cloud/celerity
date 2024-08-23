package corefunctions

import (
	"context"
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

// MapFunction provides the implementation of the
// core "map" function defined in the blueprint specification.
type MapFunction struct {
	definition *function.Definition
}

// NewMapFunction creates a new instance of the MapFunction with
// a complete function definition.
func NewMapFunction() provider.Function {
	return &MapFunction{
		definition: &function.Definition{
			Description: "Maps a list of values to a new list of values using a provided function.",
			FormattedDescription: "Maps a list of values to a new list of values using a provided function.\n\n" +
				"**Examples:**\n\n" +
				"```\n${map(\n  datasources.network.subnets,\n  compose(to_upper, getattr(\"id\")\n)}\n```",
			Parameters: []function.Parameter{
				&function.ListParameter{
					ElementType: &function.ValueTypeDefinitionAny{
						Label:       "Any",
						Type:        function.ValueTypeAny,
						Description: "A value of any type, every element in the containing list must be of the same type.",
					},
					Description: "An array of items where all items are of the same type to map.",
				},
			},
			Return: &function.ListReturn{
				ElementType: &function.ValueTypeDefinitionAny{
					Label:       "Any",
					Type:        function.ValueTypeAny,
					Description: "A value of any type, every element in the returned list must be of the same type.",
				},
				Description: "The list of values after applying the mapping function.",
			},
		},
	}
}

func (f *MapFunction) GetDefinition(
	ctx context.Context,
	input *provider.FunctionGetDefinitionInput,
) (*provider.FunctionGetDefinitionOutput, error) {
	return &provider.FunctionGetDefinitionOutput{
		Definition: f.definition,
	}, nil
}

func (f *MapFunction) Call(
	ctx context.Context,
	input *provider.FunctionCallInput,
) (*provider.FunctionCallOutput, error) {
	var items []interface{}
	var mapFuncInfo provider.FunctionRuntimeInfo
	if err := input.Arguments.GetMultipleVars(ctx, &items, &mapFuncInfo); err != nil {
		return nil, err
	}

	// It would be very costly to check the type of each item in the list
	// at this stage, so we will pass each item to the provided function
	// and trust the function will check the type and return an error
	// when it encounters an item of the wrong type.
	newItems := make([]interface{}, len(items))
	for i, item := range items {
		callArgs := []interface{}{item}
		if mapFuncInfo.ArgsOffset == 1 {
			callArgs = append(callArgs, mapFuncInfo.PartialArgs...)
		} else if mapFuncInfo.ArgsOffset > 1 {
			return nil, function.NewFuncCallError(
				fmt.Sprintf(
					"invalid args offset defined for "+
						"the partially applied \"%s\" function, "+
						"this is an issue with the function used to "+
						"create the function value passed into map",
					mapFuncInfo.FunctionName,
				),
				function.FuncCallErrorCodeInvalidArgsOffset,
				input.CallContext.CallStackSnapshot(),
			)
		} else {
			callArgs = append(mapFuncInfo.PartialArgs, callArgs...)
		}

		output, err := input.CallContext.Registry().Call(
			ctx,
			mapFuncInfo.FunctionName,
			&provider.FunctionCallInput{
				Arguments: input.CallContext.NewCallArgs(callArgs...),
			},
		)
		if err != nil {
			return nil, err
		}

		newItems[i] = output.ResponseData
	}

	return &provider.FunctionCallOutput{
		ResponseData: newItems,
	}, nil
}

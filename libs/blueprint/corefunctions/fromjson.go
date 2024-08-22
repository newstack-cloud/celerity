package corefunctions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

// FromJSONFunction provides the implementation of the
// core "fromjson" function defined in the blueprint specification.
type FromJSONFunction struct {
	definition *function.Definition
}

// NewFromJSONFunction creates a new instance of the FromJSONFunction with
// a complete function definition.
func NewFromJSONFunction() provider.Function {
	return &FromJSONFunction{
		definition: &function.Definition{
			Description: "Extracts a value from a serialised JSON string. " +
				"This uses json pointer notation to allow for the extraction of values from complex " +
				"serialised structures.",
			FormattedDescription: "Extracts a value from a serialised JSON string. " +
				"This uses [json pointer notation](https://datatracker.ietf.org/doc/rfc6901/) " +
				"to allow for the extraction of values from complex serialised structures.\n\n" +
				"**Examples:**\n\n" +
				"```\n${fromjson(variables.cacheClusterConfig, \"/host\")}\n```",
			Parameters: []function.Parameter{
				&function.ScalarParameter{
					Type: &function.ValueTypeDefinitionScalar{
						Label: "JSON string",
						Type:  function.ValueTypeString,
						Description: "A valid string literal, reference or function" +
							" call yielding the json string to extract values from",
					},
				},
				&function.ScalarParameter{
					Type: &function.ValueTypeDefinitionScalar{
						Label:       "JSON pointer",
						Type:        function.ValueTypeString,
						Description: "A valid json pointer expression to extract the value from the json string.",
						FormattedDescription: "A valid [json pointer expression](https://datatracker.ietf.org/doc/rfc6901/) " +
							"to extract the value from the json string.",
					},
				},
			},
			Return: &function.AnyReturn{
				Type: function.ValueTypeAny,
				Description: "The value extracted from the json string. " +
					"This can be a primitive value, an array, mapping or object",
			},
		},
	}
}

func (f *FromJSONFunction) GetDefinition(
	ctx context.Context,
	input *provider.FunctionGetDefinitionInput,
) (*provider.FunctionGetDefinitionOutput, error) {
	return &provider.FunctionGetDefinitionOutput{
		Definition: f.definition,
	}, nil
}

func (f *FromJSONFunction) Call(
	ctx context.Context,
	input *provider.FunctionCallInput,
) (*provider.FunctionCallOutput, error) {
	var jsonStr string
	var jsonPtr string
	err := input.Arguments.GetMultipleVars(ctx, &jsonStr, &jsonPtr)
	if err != nil {
		return nil, err
	}

	decoded := make(map[string]interface{})
	err = json.Unmarshal([]byte(jsonStr), &decoded)
	if err != nil {
		return nil, err
	}

	value, ok := extractJSONValueWithPointer(decoded, jsonPtr)
	if !ok {
		return nil, fmt.Errorf("unable to extract value from json string using pointer %s", jsonPtr)
	}

	return &provider.FunctionCallOutput{
		ResponseData: value,
	}, nil
}

func extractJSONValueWithPointer(data map[string]interface{}, pointer string) (interface{}, bool) {
	if pointer == "" {
		return data, true
	}

	pointerParts := strings.Split(pointer, "/")
	current := data
	found := false

	i := 0
	for !found && i < len(pointerParts) {
		i += 1
	}

	return current, found
}

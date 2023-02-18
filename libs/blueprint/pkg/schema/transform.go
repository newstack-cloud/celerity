package schema

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/freshwebio/celerity/libs/common/pkg/core"
	"gopkg.in/yaml.v3"
)

// TransformValueWrapper holds one or more transforms
// to be applied to a specification.
// This allows for users to provide the transform field in a spec
// as a string or as a list of strings.
type TransformValueWrapper struct {
	Values []string
}

func (t *TransformValueWrapper) MarshalYAML() (interface{}, error) {
	// Always marshal as a slice.
	return t.Values, nil
}

func (t *TransformValueWrapper) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		t.Values = []string{value.Value}
		return nil
	}

	if value.Kind == yaml.SequenceNode {
		values, err := collectStringNodeValues(value.Content)
		if err != nil {
			return err
		}
		t.Values = values
		return nil
	}

	return errInvalidTransformType(
		fmt.Errorf("unexpected yaml node for transform: %s", yamlKindMappings[value.Kind]),
	)
}

func (t *TransformValueWrapper) MarshalJSON() ([]byte, error) {
	// Always marshal as a slice.
	return json.Marshal(t.Values)
}

func (t *TransformValueWrapper) UnmarshalJSON(data []byte) error {
	transformValues := []string{}
	// Try to parse a slice, then fall back to a single string.
	// There is no better way to know with the built-in JSON library,
	// yes there are more efficient checks you can do by simply looking
	// at the characters in the string but they will not be as reliable
	// as unmarshalling.
	err := json.Unmarshal(data, &transformValues)
	if err == nil {
		t.Values = transformValues
		return nil
	}

	var transformValue string
	err = json.Unmarshal(data, &transformValue)
	if err != nil {
		return errInvalidTransformType(
			fmt.Errorf("unexpected value provided for transform in json: %s", err.Error()),
		)
	}
	t.Values = []string{transformValue}
	return nil
}

func collectStringNodeValues(nodes []*yaml.Node) ([]string, error) {
	values := []string{}
	// For at least 99% of the cases it will be trivial to go through
	// the entire list of transform value nodes and identify any invalid
	// values. This is much better for users of the spec too!
	nonScalarNodeKinds := []yaml.Kind{}
	for _, node := range nodes {
		if node.Kind != yaml.ScalarNode {
			nonScalarNodeKinds = append(nonScalarNodeKinds, node.Kind)
		} else {
			values = append(values, node.Value)
		}
	}

	if len(nonScalarNodeKinds) > 0 {
		return nil, errInvalidTransformType(
			fmt.Errorf(
				"unexpected yaml nodes in transform list, only scalars are supported: %s",
				formatYamlNodeKindsForError(nonScalarNodeKinds),
			),
		)
	}

	return values, nil
}

func formatYamlNodeKindsForError(nodeKinds []yaml.Kind) string {
	return strings.Join(
		core.Map(nodeKinds, func(kind yaml.Kind, index int) string {
			return fmt.Sprintf("%d:%s", index, yamlKindMappings[kind])
		}),
		",",
	)
}

var yamlKindMappings = map[yaml.Kind]string{
	yaml.AliasNode:    "alias",
	yaml.DocumentNode: "document",
	yaml.ScalarNode:   "scalar",
	yaml.MappingNode:  "mapping",
	yaml.SequenceNode: "sequence",
}

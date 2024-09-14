package blueprint

import "gopkg.in/yaml.v3"

// ParseYAMLNode parses the given YAML content and returns the root node
// of the YAML document hierarchy that can be used for things like extracting
// document symbols.
func ParseYAMLNode(content string) (*yaml.Node, error) {
	var node yaml.Node

	err := yaml.Unmarshal([]byte(content), &node)
	if err != nil {
		return nil, err
	}

	return &node, nil
}

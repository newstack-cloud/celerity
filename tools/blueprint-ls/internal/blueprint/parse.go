package blueprint

import (
	"github.com/coreos/go-json"
	"github.com/tailscale/hujson"
	"gopkg.in/yaml.v3"
)

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

// ParseJWCCNode parses the given JSON with Commas and Comments
// content and returns the root node of the JSON document hierarchy
// that can be used for things like extracting document symbols.
func ParseJWCCNode(content string) (*json.Node, error) {
	var node json.Node

	standardised, err := hujson.Standardize([]byte(content))
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(standardised, &node)
	if err != nil {
		return nil, err
	}

	return &node, nil
}

package core

import json "github.com/coreos/go-json"

// JSONNodeExtractable is an interface that allows
// a struct to be populated from a JSON node with offsets.
type JSONNodeExtractable interface {
	// FromJSONNode populates the struct from a JSON node.
	// The linePositions parameter is a slice of integers
	// that represents the line offsets in the entire source
	// document.
	// The parentPath parameter is a string that represents the path
	// to the parent node in the JSON document that is used
	// to provide extra context for errors.
	FromJSONNode(node *json.Node, linePositions []int, parentPath string) error
}

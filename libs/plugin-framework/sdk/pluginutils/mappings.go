package pluginutils

import "github.com/newstack-cloud/celerity/libs/blueprint/core"

// GetValueByPath is a helper function to extract a value from a mapping node
// that is a thin wrapper around the blueprint framework's `core.GetPathValue` function.
// Unlike core.GetPathValue, this function will not return an error,
// instead it will return nil and false if the value is not found
// or the provided path is not valid.
func GetValueByPath(
	fieldPath string,
	specData *core.MappingNode,
) (*core.MappingNode, bool) {
	value, err := core.GetPathValue(
		fieldPath,
		specData,
		core.MappingNodeMaxTraverseDepth,
	)
	if err != nil {
		return nil, false
	}

	return value, value != nil
}

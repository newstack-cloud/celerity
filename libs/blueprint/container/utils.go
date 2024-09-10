package container

import (
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
)

func deriveSpecFormat(specFilePath string) (schema.SpecFormat, error) {
	// Bear in mind this is a somewhat naive check, however if the spec file data
	// isn't valid YAML or JSON it will be caught in a failure to unmarshal
	// the spec.
	if strings.HasSuffix(specFilePath, ".yml") || strings.HasSuffix(specFilePath, ".yaml") {
		return schema.YAMLSpecFormat, nil
	}

	if strings.HasSuffix(specFilePath, ".json") {
		return schema.JSONSpecFormat, nil
	}

	return "", errUnsupportedSpecFileExtension(specFilePath)
}

// Provide a function compatible with loadSpec that simply returns an already defined format.
// This is useful for using the same functionality for loading from a string and from disk.
func predefinedFormatFactory(predefinedFormat schema.SpecFormat) func(input string) (schema.SpecFormat, error) {
	return func(input string) (schema.SpecFormat, error) {
		return predefinedFormat, nil
	}
}

func copyProviderMap(m map[string]provider.Provider) map[string]provider.Provider {
	copy := make(map[string]provider.Provider, len(m))
	for k, v := range m {
		copy[k] = v
	}
	return copy
}

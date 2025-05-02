package helpersv1

import (
	"regexp"

	"github.com/two-hundred/celerity/libs/blueprint/includes"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
)

var (
	yamlFilePattern = regexp.MustCompile(`\.ya?ml$`)
)

// GetFormat determines the format of the blueprint file based on its extension.
func GetFormat(
	blueprintFileName string,
) schema.SpecFormat {
	if yamlFilePattern.MatchString(blueprintFileName) {
		return schema.YAMLSpecFormat
	}

	// Any other file extension will be considered JSON.
	return schema.JSONSpecFormat
}

// GetBlueprintSource retrieves the source of the blueprint from the provided
// ChildBlueprintInfo. If the source is not set, it returns an empty string.
func GetBlueprintSource(
	blueprintInfo *includes.ChildBlueprintInfo,
) string {
	if blueprintInfo == nil || blueprintInfo.BlueprintSource == nil {
		return ""
	}

	return *blueprintInfo.BlueprintSource
}

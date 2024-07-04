package validation

import (
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/source"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/substitutions"
)

// ValidateResourceName checks the validity of a resource name,
// primarily making sure that it does not contain any substitutions
// as per the spec.
func ValidateResourceName(mappingName string, resourceMap *schema.ResourceMap) error {
	if substitutions.ContainsSubstitution(mappingName) {
		return errMappingNameContainsSubstitution(
			mappingName,
			"resource",
			ErrorReasonCodeInvalidResource,
			getResourceSourceMeta(resourceMap, mappingName),
		)
	}
	return nil
}

func getResourceSourceMeta(resourceMap *schema.ResourceMap, resourceName string) *source.Meta {
	if resourceMap == nil {
		return nil
	}

	return resourceMap.SourceMeta[resourceName]
}

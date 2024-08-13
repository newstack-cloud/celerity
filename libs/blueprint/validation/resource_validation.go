package validation

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
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

// PreValidateResourceSpec pre-validates the resource specification against the blueprint
// specification. This primarily searches for invalid usage of substitutions in mapping keys.
// The main resource validation that invokes a user-provided resource implementation
// comes after this.
func PreValidateResourceSpec(
	ctx context.Context,
	resourceName string,
	resourceSchema *schema.Resource,
	resourceMap *schema.ResourceMap,
) error {
	if resourceSchema.Spec == nil {
		return nil
	}

	errors := preValidateMappingNode(ctx, resourceSchema.Spec, "resource", resourceName)
	if len(errors) > 0 {
		return errResourceSpecPreValidationFailed(
			errors,
			resourceName,
			getResourceSourceMeta(resourceMap, resourceName),
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

package container

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
)

// ExpandResourceTemplates expands resource templates in a parsed blueprint.
// This function carries out the following work:
//   - Resolves the `each` input for resource templates.
//   - Converts a resource template into individual resources in the blueprint.
//   - Adjusts link selectors and labels for each resource derived from a template.
//   - Caches the resolved items for the `each` property of a resource template
//     so they can be used to resolve each resource derived from the template later.
//
// The following link relationships are supported for resource templates:
//
//   - A regular resource links to a resource template.
//     The labels from the resource template are applied to each expanded resource.
//
//   - A resource template links to a regular resource.
//     The link selector from the resource template is applied to each expanded resource.
//
//   - A resource template links to another resource template where the resolved items
//     list is of the same length.
//     In the following definition, "RT" stands for resource template.
//     Link selectors in RT(a) that correspend to labels in RT(b) are updated to include
//     an index to match the resource at the same index in RT(b).
//     Labels in RT(b) that correspond to link selectors in RT(a) are updated to include
//     an index to allow the resource from RT(a) to select the resource from RT(b).
//
// Links between resource templates of different lengths are not supported,
// this will result in an error during an attempt to expand the resource templates.
// This error has to be determined at runtime and not at the validation stage
// because the length of the resolved items for a resource template is not known
// until the value of the `each` property is resolved.
func ExpandResourceTemplates(
	ctx context.Context,
	blueprint *schema.Blueprint,
	substitutionResolver subengine.SubstitutionResolver,
	cache *core.Cache[[]*core.MappingNode],
) (*schema.Blueprint, error) {
	return blueprint, nil
}

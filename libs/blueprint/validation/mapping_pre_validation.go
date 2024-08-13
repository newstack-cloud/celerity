package validation

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
)

func preValidateMappingNode(
	ctx context.Context,
	node *core.MappingNode,
	nodeParentType string,
	nodeParentName string,
) []error {
	errors := []error{}
	preValidateMappingNodeRecursive(ctx, node, nodeParentType, nodeParentName, 0, &errors)
	return errors
}

func preValidateMappingNodeRecursive(
	ctx context.Context,
	node *core.MappingNode,
	nodeParentType string,
	nodeParentName string,
	depth int,
	errors *[]error,
) {
	if depth > MappingNodeMaxTraverseDepth {
		return
	}

	if node.Fields != nil {
		for key, value := range node.Fields {
			if substitutions.ContainsSubstitution(key) {
				*errors = append(
					*errors,
					errMappingNodeKeyContainsSubstitution(
						key,
						nodeParentType,
						nodeParentName,
						node.SourceMeta,
					),
				)
			}

			if value.Fields != nil || value.Items != nil {
				preValidateMappingNodeRecursive(
					ctx, value, nodeParentType, nodeParentName, depth+1, errors,
				)
			}
		}
	}

	if node.Items != nil {
		for _, item := range node.Items {
			if item.Fields != nil || item.Items != nil {
				preValidateMappingNodeRecursive(
					ctx, item, nodeParentType, nodeParentName, depth+1, errors,
				)
			}
		}
	}
}

package docgen

import (
	"fmt"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
	"github.com/two-hundred/celerity/tools/plugin-docgen/internal/utils"
)

func createProviderContext(
	namespace string,
) provider.Context {
	return provider.NewProviderContextFromParams(
		namespace,
		utils.CreateEmptyBlueprintParams(),
	)
}

func createTransformerContext(
	namespace string,
) transform.Context {
	return transform.NewTransformerContextFromParams(
		namespace,
		utils.CreateEmptyBlueprintParams(),
	)
}

func createLinkContext() provider.LinkContext {
	return provider.NewLinkContextFromParams(
		utils.CreateEmptyBlueprintParams(),
	)
}

func truncateDescription(description string, maxChars int) string {
	if len(description) > maxChars {
		// Find the last full stop or space before the maxChars limit.
		// This does not guarantee that markdown will be valid, so generally,
		// it is best to populate the "summary" fields when implementing plugins.
		lastSpace := maxChars
		for i := maxChars; i >= 0; i-- {
			if description[i] == ' ' {
				lastSpace = i
				break
			}
		}
		return description[:lastSpace] + "..."
	}
	return description
}

type linkTypeInfo struct {
	resourceTypeA string
	resourceTypeB string
}

func extractLinkTypeInfo(linkType string) (*linkTypeInfo, error) {
	parts := strings.Split(linkType, "::")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid link type: %s", linkType)
	}

	return &linkTypeInfo{
		resourceTypeA: parts[0],
		resourceTypeB: parts[1],
	}, nil
}

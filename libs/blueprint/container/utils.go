package container

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
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

func collectLinksFromChain(
	ctx context.Context,
	chain *links.ChainLinkNode,
	refChainCollector validation.RefChainCollector,
) error {
	referencedByResourceID := fmt.Sprintf("resources.%s", chain.ResourceName)
	for _, link := range chain.LinksTo {
		linkImplementation, err := getLinkImplementation(chain, link)
		if err != nil {
			return err
		}

		linkKindOutput, err := linkImplementation.GetKind(ctx, &provider.LinkGetKindInput{})
		if err != nil {
			return err
		}

		if !alreadyCollected(refChainCollector, link, referencedByResourceID) {
			err = collectResourceFromLink(ctx, refChainCollector, link, linkKindOutput.Kind, referencedByResourceID)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func collectResourceFromLink(
	ctx context.Context,
	refChainCollector validation.RefChainCollector,
	link *links.ChainLinkNode,
	linkKind provider.LinkKind,
	referencedByResourceID string,
) error {
	// Only collect link for cycle detection if it is a hard link.
	// Soft links do not require a specific order of deployment/resolution.
	if linkKind == provider.LinkKindHard {
		resourceID := fmt.Sprintf("resources.%s", link.ResourceName)
		err := refChainCollector.Collect(resourceID, link, referencedByResourceID, []string{"link"})
		if err != nil {
			return err
		}
	}

	// There is no risk of infinite recursion due to cyclic links as at this point,
	// any pure link cycles have been detected and reported.
	err := collectLinksFromChain(ctx, link, refChainCollector)
	if err != nil {
		return err
	}

	return nil
}

func alreadyCollected(
	refChainCollector validation.RefChainCollector,
	link *links.ChainLinkNode,
	referencedByResourceID string,
) bool {
	elementName := fmt.Sprintf("resources.%s", link.ResourceName)
	collected := refChainCollector.Chain(elementName)
	return collected != nil &&
		slices.ContainsFunc(
			collected.ReferencedBy,
			func(current *validation.ReferenceChainNode) bool {
				return current.ElementName == referencedByResourceID
			},
		)
}

func getLinkImplementation(
	linkA *links.ChainLinkNode,
	linkB *links.ChainLinkNode,
) (provider.Link, error) {
	linkImplementation, hasLinkImplementation := linkA.LinkImplementations[linkB.ResourceName]
	if !hasLinkImplementation {
		// The relationship could be either way.
		linkImplementation, hasLinkImplementation = linkB.LinkImplementations[linkA.ResourceName]
	}

	if !hasLinkImplementation {
		return nil, fmt.Errorf("no link implementation found between %s and %s", linkA.ResourceName, linkB.ResourceName)
	}

	return linkImplementation, nil
}

func extractChildRefNodes(
	blueprint *schema.Blueprint,
	refChainCollector validation.RefChainCollector,
) []*validation.ReferenceChainNode {
	childRefNodes := []*validation.ReferenceChainNode{}
	if blueprint.Include == nil {
		return childRefNodes
	}

	for childName, _ := range blueprint.Include.Values {
		refChainNode := refChainCollector.Chain(fmt.Sprintf("children.%s", childName))
		if refChainNode != nil {
			childRefNodes = append(childRefNodes, refChainNode)
		}
	}

	return childRefNodes
}

func extractIncludeVariables(include *subengine.ResolvedInclude) map[string]*core.ScalarValue {
	includeVariables := map[string]*core.ScalarValue{}

	if include == nil || include.Variables == nil {
		return includeVariables
	}

	for variableName, variableValue := range include.Variables.Fields {
		if variableValue.Literal != nil {
			includeVariables[variableName] = variableValue.Literal
		}
	}

	return includeVariables
}

func extractChildBlueprintFormat(includeName string, include *subengine.ResolvedInclude) (schema.SpecFormat, error) {
	if include == nil || include.Path == nil {
		return schema.SpecFormat(""), errMissingChildBlueprintPath(includeName)
	}

	pathString := core.StringValue(include.Path)
	if pathString == "" {
		// This should lead to an error when trying to load a child blueprint.
		return schema.SpecFormat(""), errEmptyChildBlueprintPath(includeName)
	}

	return deriveSpecFormat(pathString)
}

func createLinkID(resourceAName string, resourceBName string) string {
	return fmt.Sprintf(
		"%s::%s",
		resourceAName,
		resourceBName,
	)
}

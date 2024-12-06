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
	"github.com/two-hundred/celerity/libs/blueprint/speccore"
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
	referencedByResourceID := core.ResourceElementID(chain.ResourceName)
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
		resourceID := core.ResourceElementID(link.ResourceName)
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
	elementName := core.ResourceElementID(link.ResourceName)
	collected := refChainCollector.Chain(elementName)
	return collected != nil &&
		slices.ContainsFunc(
			collected.ReferencedBy,
			func(current *validation.ReferenceChainNode) bool {
				return current.ElementName == referencedByResourceID
			},
		)
}

func getChangesFromStageLinkChangesOutput(
	stageLinkChangesOutput *provider.LinkStageChangesOutput,
) provider.LinkChanges {
	if stageLinkChangesOutput == nil || stageLinkChangesOutput.Changes == nil {
		return provider.LinkChanges{}
	}

	return *stageLinkChangesOutput.Changes
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

	for childName := range blueprint.Include.Values {
		refChainNode := refChainCollector.Chain(core.ChildElementID(childName))
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
		if variableValue.Scalar != nil {
			includeVariables[variableName] = variableValue.Scalar
		}
	}

	return includeVariables
}

func isConditionKnownOnDeploy(
	resourceName string,
	resolveOnDeploy []string,
) bool {
	resourceElementID := core.ResourceElementID(resourceName)
	return slices.ContainsFunc(resolveOnDeploy, func(resolveOnDeployProp string) bool {
		conditionPropPrefix := fmt.Sprintf("%s.condition", resourceElementID)
		return strings.HasPrefix(resolveOnDeployProp, conditionPropPrefix)
	})
}

func evaluateCondition(
	condition *provider.ResolvedResourceCondition,
) bool {
	if condition == nil {
		return true
	}

	if condition.And != nil {
		result := true
		for _, subCondition := range condition.And {
			result = result && evaluateCondition(subCondition)
		}
		return result
	}

	if condition.Or != nil {
		result := false
		for _, subCondition := range condition.Or {
			result = result || evaluateCondition(subCondition)
		}
		return result
	}

	if condition.Not != nil {
		return !evaluateCondition(condition.Not)
	}

	if condition.StringValue != nil {
		return core.BoolValue(condition.StringValue)
	}

	// A condition with no value set is equivalent to a condition not being set at all
	// for the given resource.
	return true
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

func flattenMapLists[Value any](m map[string][]Value) []Value {
	flattened := []Value{}
	for _, list := range m {
		flattened = append(flattened, list...)
	}
	return flattened
}

func createLinkID(resourceAName string, resourceBName string) string {
	return fmt.Sprintf(
		"%s::%s",
		resourceAName,
		resourceBName,
	)
}

func getInstanceTreePath(
	params core.BlueprintParams,
	instanceID string,
) string {
	instanceTreePath := params.ContextVariable("instanceTreePath")
	if instanceTreePath == nil || instanceTreePath.StringValue == nil {
		return instanceID
	}

	parentTreePath := *instanceTreePath.StringValue
	return addToInstanceTreePath(parentTreePath, instanceID)
}

func addToInstanceTreePath(
	parentInstanceTreePath string,
	instanceID string,
) string {
	if parentInstanceTreePath == "" {
		return instanceID
	}

	return fmt.Sprintf("%s/%s", parentInstanceTreePath, instanceID)
}

func getIncludeTreePath(
	params core.BlueprintParams,
	includeChildIDName string,
) string {
	childName := strings.TrimPrefix(includeChildIDName, "children.")
	includeName := ""
	if childName != "" {
		includeName = fmt.Sprintf("include.%s", childName)
	}
	includeTreePath := params.ContextVariable("includeTreePath")
	if includeTreePath == nil || includeTreePath.StringValue == nil {
		return includeName
	}

	parentTreePath := *includeTreePath.StringValue
	return addToIncludeTreePath(parentTreePath, includeName)
}

func addToIncludeTreePath(
	parentIncludeTreePath string,
	includeName string,
) string {
	if parentIncludeTreePath == "" {
		return includeName
	}

	if includeName == "" {
		return parentIncludeTreePath
	}

	return fmt.Sprintf("%s::%s", parentIncludeTreePath, includeName)
}

func hasBlueprintCycle(
	parentInstanceTreePath string,
	instanceID string,
) bool {
	if parentInstanceTreePath == "" || instanceID == "" {
		return false
	}

	instances := strings.Split(parentInstanceTreePath, "/")
	return slices.Contains(instances, instanceID)
}

func createContextVarsForChildBlueprint(
	parentInstanceID string,
	instanceTreePath string,
	includeTreePath string,
) map[string]*core.ScalarValue {
	return map[string]*core.ScalarValue{
		"parentInstanceID": {
			StringValue: &parentInstanceID,
		},
		"instanceTreePath": {
			StringValue: &instanceTreePath,
		},
		"includeTreePath": {
			StringValue: &includeTreePath,
		},
	}
}

func createResourceTypeProviderMap(
	blueprintSpec speccore.BlueprintSpec,
	providers map[string]provider.Provider,
) map[string]provider.Provider {
	resourceTypeProviderMap := map[string]provider.Provider{}
	resources := map[string]*schema.Resource{}
	if blueprintSpec.Schema().Resources != nil {
		resources = blueprintSpec.Schema().Resources.Values
	}

	for _, resource := range resources {
		namespace := strings.Split(resource.Type.Value, "/")[0]
		resourceTypeProviderMap[resource.Type.Value] = providers[namespace]
	}
	return resourceTypeProviderMap
}

func createResourceProviderMap(
	blueprintSpec speccore.BlueprintSpec,
	providers map[string]provider.Provider,
) map[string]provider.Provider {
	resourceProviderMap := map[string]provider.Provider{}
	resources := map[string]*schema.Resource{}
	if blueprintSpec.Schema().Resources != nil {
		resources = blueprintSpec.Schema().Resources.Values
	}

	for resourceName, resource := range resources {
		namespace := strings.Split(resource.Type.Value, "/")[0]
		resourceProviderMap[resourceName] = providers[namespace]
	}
	return resourceProviderMap
}

func copyPointerMap[Item any](input map[string]*Item) map[string]Item {
	output := map[string]Item{}
	for key, value := range input {
		output[key] = *value
	}
	return output
}

func exceedsMaxDepth(path string, maxDepth int) bool {
	return len(strings.Split(path, "/")) > maxDepth
}

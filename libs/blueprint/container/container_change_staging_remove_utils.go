package container

import "github.com/two-hundred/celerity/libs/blueprint/state"

func getInstanceRemovalChanges(instance *state.InstanceState) BlueprintChanges {
	removedResources := getResourceNamesFromInstanceState(instance)
	removedLinks := getLinkIDsFromInstanceState(instance)
	childRemovalInfo := getChildRemovalInfoFromInstanceState(instance)
	removedExports := getExportNamesFromInstanceState(instance)

	return BlueprintChanges{
		RemovedResources: removedResources,
		RemovedLinks:     removedLinks,
		// Capture both the names of the children that will be removed
		// and the changes that will be applied to components of the child blueprints.
		RemovedChildren: childRemovalInfo.removedChildren,
		ChildChanges:    childRemovalInfo.childChanges,
		RemovedExports:  removedExports,
	}
}

func getResourceNamesFromInstanceState(instance *state.InstanceState) []string {
	names := make([]string, 0)
	for _, resource := range instance.Resources {
		names = append(names, resource.ResourceName)
	}
	return names
}

func getLinkIDsFromInstanceState(instance *state.InstanceState) []string {
	ids := make([]string, 0)
	for _, link := range instance.Links {
		ids = append(ids, link.LinkID)
	}
	return ids
}

func getExportNamesFromInstanceState(instance *state.InstanceState) []string {
	names := make([]string, 0)
	for exportName := range instance.Exports {
		names = append(names, exportName)
	}
	return names
}

func getChildRemovalInfoFromInstanceState(instance *state.InstanceState) *childBlueprintRemovalInfo {
	removalInfo := &childBlueprintRemovalInfo{
		removedChildren: []string{},
		childChanges:    map[string]BlueprintChanges{},
	}
	for childName, child := range instance.ChildBlueprints {
		removalInfo.removedChildren = append(removalInfo.removedChildren, childName)
		removalInfo.childChanges[childName] = getInstanceRemovalChanges(child)
	}
	return removalInfo
}

type childBlueprintRemovalInfo struct {
	removedChildren []string
	childChanges    map[string]BlueprintChanges
}

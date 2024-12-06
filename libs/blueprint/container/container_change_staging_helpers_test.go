package container

import (
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

func createChangeStagingChannels() *ChangeStagingChannels {
	return &ChangeStagingChannels{
		ResourceChangesChan: make(chan ResourceChangesMessage),
		ChildChangesChan:    make(chan ChildChangesMessage),
		LinkChangesChan:     make(chan LinkChangesMessage),
		CompleteChan:        make(chan BlueprintChanges),
		ErrChan:             make(chan error),
	}
}

func normaliseBlueprintChanges(changes *BlueprintChanges) *BlueprintChanges {

	normalisedChanges := &BlueprintChanges{
		NewResources:     normaliseResourceChangeMap(changes.NewResources),
		ResourceChanges:  normaliseResourceChangeMap(changes.ResourceChanges),
		RemovedResources: internal.OrderStringSlice(changes.RemovedResources),
		RemovedLinks:     internal.OrderStringSlice(changes.RemovedLinks),
		NewChildren:      normaliseNewChildMap(changes.NewChildren),
		ChildChanges:     normaliseChildChangesMap(changes.ChildChanges),
		NewExports:       changes.NewExports,
		ExportChanges:    changes.ExportChanges,
		RemovedExports:   internal.OrderStringSlice(changes.RemovedExports),
		UnchangedExports: internal.OrderStringSlice(changes.UnchangedExports),
		ResolveOnDeploy:  internal.OrderStringSlice(changes.ResolveOnDeploy),
	}

	return normalisedChanges
}

func normaliseChildChangesMap(
	childChangesMap map[string]BlueprintChanges,
) map[string]BlueprintChanges {
	newMap := map[string]BlueprintChanges{}
	for childName, child := range childChangesMap {
		newMap[childName] = *normaliseBlueprintChanges(&child)
	}
	return newMap
}

func normaliseResourceChangeMap(changeMap map[string]provider.Changes) map[string]provider.Changes {
	newChangeMap := map[string]provider.Changes{}
	for resourceName, resourceChange := range changeMap {
		newChangeMap[resourceName] = *internal.NormaliseResourceChanges(&resourceChange, false)
	}
	return newChangeMap
}

func normaliseNewChildMap(newChildMap map[string]NewBlueprintDefinition) map[string]NewBlueprintDefinition {
	newMap := map[string]NewBlueprintDefinition{}
	for childName, child := range newChildMap {
		newMap[childName] = NewBlueprintDefinition{
			NewResources: normaliseResourceChangeMap(child.NewResources),
			NewChildren:  normaliseNewChildMap(child.NewChildren),
			NewExports:   child.NewExports,
		}
	}
	return newMap
}

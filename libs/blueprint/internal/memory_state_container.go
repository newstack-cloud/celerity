// An in-memory implementation of the StateContainer interface
// to be used for testing purposes.

package internal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

type MemoryStateContainer struct {
	instancesContainer *memoryInstancesContainer
	resourcesContainer *memoryResourcesContainer
	linksContainer     *memoryLinksContainer
	childrenContainer  *memoryChildrenContainer
	metadataContainer  *memoryMetadataContainer
	exportsContainer   *memoryExportsContainer
}

func NewMemoryStateContainer() state.Container {
	instances := map[string]*state.InstanceState{}
	resourceDrift := map[string]map[string]*state.ResourceDriftState{}

	mu := &sync.RWMutex{}
	return &MemoryStateContainer{
		instancesContainer: &memoryInstancesContainer{
			instances: instances,
			mu:        mu,
		},
		resourcesContainer: &memoryResourcesContainer{
			instances:     instances,
			resourceDrift: resourceDrift,
			mu:            mu,
		},
		linksContainer: &memoryLinksContainer{
			instances:         instances,
			instanceLinkIDMap: map[string]map[string]string{},
			mu:                mu,
		},
		childrenContainer: &memoryChildrenContainer{
			instances: instances,
			mu:        mu,
		},
		metadataContainer: &memoryMetadataContainer{
			instances: instances,
			mu:        mu,
		},
		exportsContainer: &memoryExportsContainer{
			instances: instances,
			mu:        mu,
		},
	}
}

func (c *MemoryStateContainer) Instances() state.InstancesContainer {
	return c.instancesContainer
}

func (c *MemoryStateContainer) Resources() state.ResourcesContainer {
	return c.resourcesContainer
}

func (c *MemoryStateContainer) Links() state.LinksContainer {
	return c.linksContainer
}

func (c *MemoryStateContainer) Metadata() state.MetadataContainer {
	return c.metadataContainer
}

func (c *MemoryStateContainer) Exports() state.ExportsContainer {
	return c.exportsContainer
}

func (c *MemoryStateContainer) Children() state.ChildrenContainer {
	return c.childrenContainer
}

type memoryInstancesContainer struct {
	instances map[string]*state.InstanceState
	mu        *sync.RWMutex
}

func (c *memoryInstancesContainer) Get(ctx context.Context, instanceID string) (state.InstanceState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		return copyInstance(instance, instanceID), nil
	}

	return state.InstanceState{}, state.InstanceNotFoundError(instanceID)
}

func (c *memoryInstancesContainer) Save(
	ctx context.Context,
	instanceState state.InstanceState,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.instances[instanceState.InstanceID] = &instanceState

	return nil
}

func (c *memoryInstancesContainer) UpdateStatus(
	ctx context.Context,
	instanceID string,
	statusInfo state.InstanceStatusInfo,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	instance, ok := c.instances[instanceID]
	if ok {
		instance.Status = statusInfo.Status
		if statusInfo.Durations != nil {
			instance.Durations = statusInfo.Durations
		}

		return nil
	}

	return state.InstanceNotFoundError(instanceID)
}

func (c *memoryInstancesContainer) Remove(ctx context.Context, instanceID string) (state.InstanceState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	instance, ok := c.instances[instanceID]
	if !ok {
		return state.InstanceState{}, state.InstanceNotFoundError(instanceID)
	}

	delete(c.instances, instanceID)
	return *instance, nil
}

type memoryResourcesContainer struct {
	instances     map[string]*state.InstanceState
	resourceDrift map[string]map[string]*state.ResourceDriftState
	mu            *sync.RWMutex
}

func (c *memoryResourcesContainer) Get(ctx context.Context, instanceID string, resourceID string) (state.ResourceState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			if resourceState, ok := instance.Resources[resourceID]; ok {
				return copyResource(resourceState), nil
			}
		}
	}

	return state.ResourceState{}, state.ResourceNotFoundError(resourceID)
}

func (c *memoryResourcesContainer) GetByName(
	ctx context.Context,
	instanceID string,
	resourceName string,
) (state.ResourceState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			resourceID, ok := instance.ResourceIDs[resourceName]
			if ok {
				return *instance.Resources[resourceID], nil
			}
		}
	}

	itemID := fmt.Sprintf("instance:%s:resource:%s", instanceID, resourceName)
	return state.ResourceState{}, state.ResourceNotFoundError(itemID)
}

func (c *memoryResourcesContainer) Save(
	ctx context.Context,
	instanceID string,
	resourceState state.ResourceState,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			if instance.Resources == nil {
				instance.Resources = make(map[string]*state.ResourceState)
			}
			instance.Resources[resourceState.ResourceID] = &resourceState
			if instance.ResourceIDs == nil {
				instance.ResourceIDs = make(map[string]string)
			}
			instance.ResourceIDs[resourceState.ResourceName] = resourceState.ResourceID
		} else {
			return state.ResourceNotFoundError(resourceState.ResourceID)
		}
	}

	return nil
}

func (c *memoryResourcesContainer) UpdateStatus(
	ctx context.Context,
	instanceID string,
	resourceID string,
	statusInfo state.ResourceStatusInfo,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			resource, ok := instance.Resources[resourceID]
			if ok {
				resource.Status = statusInfo.Status
				resource.PreciseStatus = statusInfo.PreciseStatus
				resource.FailureReasons = statusInfo.FailureReasons
				if statusInfo.Durations != nil {
					resource.Durations = statusInfo.Durations
				}

				return nil
			}
		}
	}

	return state.ResourceNotFoundError(resourceID)
}

func (c *memoryResourcesContainer) Remove(
	ctx context.Context,
	instanceID string,
	resourceID string,
) (state.ResourceState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			resource, ok := instance.Resources[resourceID]
			if ok {
				delete(instance.Resources, resourceID)
				return *resource, nil
			}
		}
	}

	return state.ResourceState{}, state.ResourceNotFoundError(resourceID)
}

func (c *memoryResourcesContainer) GetDrift(ctx context.Context, instanceID string, resourceID string) (state.ResourceDriftState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if driftEntries, ok := c.resourceDrift[instanceID]; ok {
		if driftEntries != nil {
			if driftState, ok := driftEntries[resourceID]; ok {
				return *driftState, nil
			}
		}
	}

	return state.ResourceDriftState{}, nil
}

func (c *memoryResourcesContainer) SaveDrift(
	ctx context.Context,
	instanceID string,
	driftState state.ResourceDriftState,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			resource, ok := instance.Resources[driftState.ResourceID]
			if ok {
				resource.Drifted = true
				resource.LastDriftDetectedTimestamp = driftState.Timestamp
			} else {
				return state.ResourceNotFoundError(driftState.ResourceID)
			}
		} else {
			return state.InstanceNotFoundError(instanceID)
		}
	} else {
		return state.InstanceNotFoundError(instanceID)
	}

	if driftEntries, ok := c.resourceDrift[instanceID]; ok {
		driftEntries[driftState.ResourceID] = &driftState
	} else {
		c.resourceDrift[instanceID] = map[string]*state.ResourceDriftState{
			driftState.ResourceID: &driftState,
		}
	}

	return nil
}

func (c *memoryResourcesContainer) RemoveDrift(
	ctx context.Context,
	instanceID string,
	resourceID string,
) (state.ResourceDriftState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			resource, ok := instance.Resources[resourceID]
			if ok {
				resource.Drifted = false
				resource.LastDriftDetectedTimestamp = nil
			} else {
				return state.ResourceDriftState{}, state.ResourceNotFoundError(resourceID)
			}
		} else {
			return state.ResourceDriftState{}, state.InstanceNotFoundError(instanceID)
		}
	} else {
		return state.ResourceDriftState{}, state.InstanceNotFoundError(instanceID)
	}

	if driftEntries, ok := c.resourceDrift[instanceID]; ok {
		if driftEntries != nil {
			driftState, ok := driftEntries[resourceID]
			if ok {
				delete(driftEntries, resourceID)
				return *driftState, nil
			}
		}
	}

	// No drift entry for a specific resource is fine,
	// indicating drift had already been removed or was never set
	// for the resource.
	return state.ResourceDriftState{}, nil
}

type memoryLinksContainer struct {
	instances         map[string]*state.InstanceState
	instanceLinkIDMap map[string]map[string]string
	mu                *sync.RWMutex
}

func (c *memoryLinksContainer) Get(ctx context.Context, instanceID string, linkID string) (state.LinkState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			hasLinkIDMap := c.hasInstanceLinkIDMap(instance)
			if !hasLinkIDMap {
				c.populateLinkIDMap(instance)
			}
			linkName := c.getLinkName(instanceID, linkID)
			if linkState, ok := instance.Links[linkName]; ok {
				return copyLink(linkState), nil
			}
		}
	}

	return state.LinkState{}, state.LinkNotFoundError(linkID)
}

func (c *memoryLinksContainer) GetByName(ctx context.Context, instanceID string, linkName string) (state.LinkState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			if linkState, ok := instance.Links[linkName]; ok {
				return copyLink(linkState), nil
			}
		}
	}

	elementID := fmt.Sprintf("instance:%s:link:%s", instanceID, linkName)
	return state.LinkState{}, state.LinkNotFoundError(elementID)
}

func (c *memoryLinksContainer) Save(ctx context.Context, instanceID string, linkState state.LinkState) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			if instance.Links == nil {
				instance.Links = make(map[string]*state.LinkState)
			}
			instance.Links[linkState.LinkName] = &linkState
			c.saveLinkIDMapEntry(instance, linkState.LinkID, linkState.LinkName)
		} else {
			return state.InstanceNotFoundError(instanceID)
		}
	}

	return nil
}

// A lock must be held before calling this function.
func (c *memoryLinksContainer) saveLinkIDMapEntry(instance *state.InstanceState, linkID string, linkName string) {
	hasLinkIDMap := c.hasInstanceLinkIDMap(instance)
	if !hasLinkIDMap {
		c.populateLinkIDMap(instance)
	}
	c.instanceLinkIDMap[instance.InstanceID][linkID] = linkName
}

func (c *memoryLinksContainer) UpdateStatus(
	ctx context.Context,
	instanceID string,
	linkID string,
	statusInfo state.LinkStatusInfo,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			hasLinkIDMap := c.hasInstanceLinkIDMap(instance)
			if !hasLinkIDMap {
				c.populateLinkIDMap(instance)
			}
			linkName := c.getLinkName(instanceID, linkID)
			link, ok := instance.Links[linkName]
			if ok {
				link.Status = statusInfo.Status
				link.PreciseStatus = statusInfo.PreciseStatus
				link.FailureReasons = statusInfo.FailureReasons
				if statusInfo.Durations != nil {
					link.Durations = statusInfo.Durations
				}

				return nil
			}
		}
	}

	return state.LinkNotFoundError(linkID)
}

func (c *memoryLinksContainer) hasInstanceLinkIDMap(instance *state.InstanceState) bool {
	_, ok := c.instanceLinkIDMap[instance.InstanceID]
	return ok
}

func (c *memoryLinksContainer) populateLinkIDMap(instance *state.InstanceState) {
	c.instanceLinkIDMap[instance.InstanceID] = map[string]string{}
	for linkName, link := range instance.Links {
		c.instanceLinkIDMap[instance.InstanceID][link.LinkID] = linkName
	}
}

func (c *memoryLinksContainer) getLinkName(instanceID string, linkID string) string {
	linkIDMap, ok := c.instanceLinkIDMap[instanceID]
	if !ok {
		return ""
	}

	return linkIDMap[linkID]
}

func (c *memoryLinksContainer) Remove(ctx context.Context, instanceID string, linkID string) (state.LinkState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			hasLinkIDMap := c.hasInstanceLinkIDMap(instance)
			if !hasLinkIDMap {
				c.populateLinkIDMap(instance)
			}
			linkName := c.getLinkName(instanceID, linkID)
			link, ok := instance.Links[linkName]
			if ok {
				delete(instance.Links, linkName)
				return *link, nil
			}
		}
	}

	return state.LinkState{}, state.LinkNotFoundError(linkID)
}

type memoryMetadataContainer struct {
	instances map[string]*state.InstanceState
	mu        *sync.RWMutex
}

func (c *memoryMetadataContainer) Get(ctx context.Context, instanceID string) (map[string]*core.MappingNode, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			return instance.Metadata, nil
		}
	}

	return nil, state.InstanceNotFoundError(instanceID)
}

func (c *memoryMetadataContainer) Save(
	ctx context.Context,
	instanceID string,
	metadata map[string]*core.MappingNode,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			instance.Metadata = metadata
		} else {
			return state.InstanceNotFoundError(instanceID)
		}
	}

	return nil
}

func (c *memoryMetadataContainer) Remove(ctx context.Context, instanceID string) (map[string]*core.MappingNode, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			metadata := instance.Metadata
			instance.Metadata = nil
			return metadata, nil
		}
	}

	return nil, state.InstanceNotFoundError(instanceID)
}

type memoryExportsContainer struct {
	instances map[string]*state.InstanceState
	mu        *sync.RWMutex
}

func (c *memoryExportsContainer) GetAll(ctx context.Context, instanceID string) (map[string]*state.ExportState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			return copyExports(instance.Exports), nil
		}
	}

	return nil, state.InstanceNotFoundError(instanceID)
}

func (c *memoryExportsContainer) Get(ctx context.Context, instanceID string, exportName string) (state.ExportState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			if export, ok := instance.Exports[exportName]; ok {
				exportCopy := copyExport(export)
				return *exportCopy, nil
			}
		}
	}

	return state.ExportState{}, errors.New("export not found")
}

func (c *memoryExportsContainer) SaveAll(
	ctx context.Context,
	instanceID string,
	exports map[string]*state.ExportState,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			instance.Exports = exports
		} else {
			return state.InstanceNotFoundError(instanceID)
		}
	}

	return nil
}

func (c *memoryExportsContainer) Save(
	ctx context.Context,
	instanceID string,
	exportName string,
	export state.ExportState,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			instance.Exports[exportName] = &export
		} else {
			return state.InstanceNotFoundError(instanceID)
		}
	}

	return nil
}

func (c *memoryExportsContainer) RemoveAll(ctx context.Context, instanceID string) (map[string]*state.ExportState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			exports := instance.Exports
			instance.Exports = nil
			return exports, nil
		}
	}

	return nil, state.InstanceNotFoundError(instanceID)
}

func (c *memoryExportsContainer) Remove(ctx context.Context, instanceID string, exportName string) (state.ExportState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			export, ok := instance.Exports[exportName]
			if ok {
				delete(instance.Exports, exportName)
				return *export, nil
			}
		}
	}

	return state.ExportState{}, errors.New("export not found")
}

type memoryChildrenContainer struct {
	instances map[string]*state.InstanceState
	mu        *sync.RWMutex
}

func (c *memoryChildrenContainer) Get(ctx context.Context, instanceID string, childName string) (state.InstanceState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			if child, ok := instance.ChildBlueprints[childName]; ok {
				return copyInstance(child, instanceID), nil
			} else {
				itemID := fmt.Sprintf("instance:%s:child:%s", instanceID, childName)
				return state.InstanceState{}, state.InstanceNotFoundError(itemID)
			}
		}
	}

	return state.InstanceState{}, state.InstanceNotFoundError(instanceID)
}

func (c *memoryChildrenContainer) Save(
	ctx context.Context,
	instanceID string,
	childName string,
	childState state.InstanceState,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			if instance.ChildBlueprints == nil {
				instance.ChildBlueprints = make(map[string]*state.InstanceState)
			}
			instance.ChildBlueprints[childName] = &childState
			c.instances[childState.InstanceID] = &childState
		} else {
			return state.InstanceNotFoundError(instanceID)
		}
	}

	return nil
}

func (c *memoryChildrenContainer) Remove(ctx context.Context, instanceID string, childName string) (state.InstanceState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			child, ok := instance.ChildBlueprints[childName]
			if ok {
				delete(instance.ChildBlueprints, childName)
				return *child, nil
			}
		}
	}

	itemID := fmt.Sprintf("instance:%s:child:%s", instanceID, childName)
	return state.InstanceState{}, state.InstanceNotFoundError(itemID)
}

func (c *memoryChildrenContainer) Attach(
	ctx context.Context,
	parentInstanceID string,
	childInstanceID string,
	childName string,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if parent, ok := c.instances[parentInstanceID]; ok {
		if child, ok := c.instances[childInstanceID]; ok {
			if parent.ChildBlueprints == nil {
				parent.ChildBlueprints = make(map[string]*state.InstanceState)
			}
			parent.ChildBlueprints[childName] = child
		} else {
			return state.InstanceNotFoundError(childInstanceID)
		}
	} else {
		return state.InstanceNotFoundError(parentInstanceID)
	}

	return nil
}

func (c *memoryChildrenContainer) SaveDependencies(
	ctx context.Context,
	instanceID string,
	childName string,
	dependencies *state.DependencyInfo,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance.ChildDependencies == nil {
			instance.ChildDependencies = make(map[string]*state.DependencyInfo)
		}
		instance.ChildDependencies[childName] = dependencies
	} else {
		return state.InstanceNotFoundError(instanceID)
	}

	return nil
}

func copyInstance(instanceState *state.InstanceState, path string) state.InstanceState {
	instanceCopy := *instanceState
	if instanceState.Resources != nil {
		instanceCopy.Resources = make(map[string]*state.ResourceState)
		for resourceID, resource := range instanceState.Resources {
			resCopy := copyResource(resource)
			instanceCopy.Resources[resourceID] = &resCopy
		}
	}
	if instanceState.ResourceIDs != nil {
		instanceCopy.ResourceIDs = make(map[string]string)
		for resourceName, resourceID := range instanceState.ResourceIDs {
			instanceCopy.ResourceIDs[resourceName] = resourceID
		}
	}
	if instanceState.Links != nil {
		instanceCopy.Links = make(map[string]*state.LinkState)
		for linkName, link := range instanceState.Links {
			linkCopy := copyLink(link)
			instanceCopy.Links[linkName] = &linkCopy
		}
	}
	if instanceState.Metadata != nil {
		instanceCopy.Metadata = make(map[string]*core.MappingNode)
		for key, value := range instanceState.Metadata {
			instanceCopy.Metadata[key] = value
		}
	}
	if instanceState.Exports != nil {
		instanceCopy.Exports = make(map[string]*state.ExportState)
		for exportName, export := range instanceState.Exports {
			exportCopy := copyExport(export)
			instanceCopy.Exports[exportName] = exportCopy
		}
	}
	if instanceState.ChildBlueprints != nil {
		instanceCopy.ChildBlueprints = make(map[string]*state.InstanceState)
		for childName, childState := range instanceState.ChildBlueprints {
			if instancePathContains(path, childState.InstanceID) {
				// Avoid circular references
				continue
			}
			copy := copyInstance(childState, fmt.Sprintf("%s/%s", path, childState.InstanceID))
			instanceCopy.ChildBlueprints[childName] = &copy
		}
	}
	if instanceState.ChildDependencies != nil {
		instanceCopy.ChildDependencies = make(map[string]*state.DependencyInfo)
		for childName, dependencyInfo := range instanceState.ChildDependencies {
			instanceCopy.ChildDependencies[childName] = copyDependencyInfo(dependencyInfo)
		}
	}
	return instanceCopy
}

func instancePathContains(path string, instanceID string) bool {
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == instanceID {
			return true
		}
	}
	return false
}

func copyResource(resourceState *state.ResourceState) state.ResourceState {
	if resourceState == nil {
		return state.ResourceState{}
	}

	metadataCopy := copyResourceMetadata(resourceState.Metadata)

	dependsOnResources := make([]string, len(resourceState.DependsOnResources))
	copy(dependsOnResources, resourceState.DependsOnResources)

	dependsOnChildren := make([]string, len(resourceState.DependsOnChildren))
	copy(dependsOnChildren, resourceState.DependsOnChildren)

	return state.ResourceState{
		ResourceID:           resourceState.ResourceID,
		ResourceName:         resourceState.ResourceName,
		ResourceType:         resourceState.ResourceType,
		ResourceTemplateName: resourceState.ResourceTemplateName,
		InstanceID:           resourceState.InstanceID,
		Status:               resourceState.Status,
		PreciseStatus:        resourceState.PreciseStatus,
		Description:          resourceState.Description,
		Metadata:             &metadataCopy,
		DependsOnResources:   dependsOnResources,
		DependsOnChildren:    dependsOnChildren,
		FailureReasons:       resourceState.FailureReasons,
		// The spec data pointer will be copied, no part of the blueprint container
		// implementation should modify the spec data in instance state so it is safe
		// to copy the pointer instead of making a deep copy.
		ResourceSpecData:           resourceState.ResourceSpecData,
		LastDeployedTimestamp:      resourceState.LastDeployedTimestamp,
		LastDeployAttemptTimestamp: resourceState.LastDeployAttemptTimestamp,
		Drifted:                    resourceState.Drifted,
		LastDriftDetectedTimestamp: resourceState.LastDriftDetectedTimestamp,
		Durations:                  resourceState.Durations,
	}
}

func copyResourceMetadata(metadata *state.ResourceMetadataState) state.ResourceMetadataState {
	if metadata == nil {
		return state.ResourceMetadataState{}
	}

	return state.ResourceMetadataState{
		DisplayName: metadata.DisplayName,
		Annotations: metadata.Annotations,
		Labels:      metadata.Labels,
		Custom:      metadata.Custom,
	}
}

func copyLink(linkState *state.LinkState) state.LinkState {
	if linkState == nil {
		return state.LinkState{}
	}

	return state.LinkState{
		LinkID:                     linkState.LinkID,
		LinkName:                   linkState.LinkName,
		InstanceID:                 linkState.InstanceID,
		Status:                     linkState.Status,
		PreciseStatus:              linkState.PreciseStatus,
		LastDeployedTimestamp:      linkState.LastDeployedTimestamp,
		LastDeployAttemptTimestamp: linkState.LastDeployAttemptTimestamp,
		IntermediaryResourceStates: copyIntermediaryResources(
			linkState.IntermediaryResourceStates,
		),
		LinkData:       linkState.LinkData,
		FailureReasons: linkState.FailureReasons,
		Durations:      linkState.Durations,
	}
}

func copyIntermediaryResources(
	intermediaryResourceStates []*state.LinkIntermediaryResourceState,
) []*state.LinkIntermediaryResourceState {
	if intermediaryResourceStates == nil {
		return nil
	}

	intermediaryResourcesCopy := []*state.LinkIntermediaryResourceState{}
	for i, value := range intermediaryResourceStates {
		intermediaryResourcesCopy[i] = &state.LinkIntermediaryResourceState{
			ResourceID:                 value.ResourceID,
			InstanceID:                 value.InstanceID,
			LastDeployedTimestamp:      value.LastDeployedTimestamp,
			LastDeployAttemptTimestamp: value.LastDeployAttemptTimestamp,
			ResourceSpecData:           value.ResourceSpecData,
		}
	}

	return intermediaryResourcesCopy
}

func copyExports(
	exports map[string]*state.ExportState,
) map[string]*state.ExportState {
	if exports == nil {
		return nil
	}

	exportsCopy := make(map[string]*state.ExportState)
	for exportName, export := range exports {
		exportCopy := copyExport(export)
		exportsCopy[exportName] = exportCopy
	}

	return exportsCopy
}

func copyExport(
	exportState *state.ExportState,
) *state.ExportState {
	if exportState == nil {
		return nil
	}

	return &state.ExportState{
		Value: exportState.Value,
		Type:  exportState.Type,
		Field: exportState.Field,
	}
}

func copyDependencyInfo(
	dependencyInfo *state.DependencyInfo,
) *state.DependencyInfo {
	if dependencyInfo == nil {
		return nil
	}

	dependsOnResources := make([]string, len(dependencyInfo.DependsOnResources))
	copy(dependsOnResources, dependencyInfo.DependsOnResources)

	dependsOnChildren := make([]string, len(dependencyInfo.DependsOnChildren))
	copy(dependsOnChildren, dependencyInfo.DependsOnChildren)

	return &state.DependencyInfo{
		DependsOnResources: dependsOnResources,
		DependsOnChildren:  dependsOnChildren,
	}
}

// An in-memory implementation of the StateContainer interface
// to be used for testing purposes.

package internal

import (
	"context"
	"errors"
	"fmt"
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
			instances: instances,
			mu:        mu,
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
		return *instance, nil
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
				return *resourceState, nil
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

	return state.ResourceDriftState{}, state.ResourceNotFoundError(resourceID)
}

type memoryLinksContainer struct {
	instances map[string]*state.InstanceState
	mu        *sync.RWMutex
}

func (c *memoryLinksContainer) Get(ctx context.Context, instanceID string, linkID string) (state.LinkState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			if linkState, ok := instance.Links[linkID]; ok {
				return *linkState, nil
			}
		}
	}

	return state.LinkState{}, state.LinkNotFoundError(linkID)
}

func (c *memoryLinksContainer) Save(ctx context.Context, instanceID string, linkState state.LinkState) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			instance.Links[linkState.LinkID] = &linkState
		} else {
			return state.InstanceNotFoundError(instanceID)
		}
	}

	return nil
}

func (c *memoryLinksContainer) Remove(ctx context.Context, instanceID string, linkID string) (state.LinkState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			link, ok := instance.Links[linkID]
			if ok {
				delete(instance.Links, linkID)
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

func (c *memoryExportsContainer) GetAll(ctx context.Context, instanceID string) (map[string]*core.MappingNode, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			return instance.Exports, nil
		}
	}

	return nil, state.InstanceNotFoundError(instanceID)
}

func (c *memoryExportsContainer) Get(ctx context.Context, instanceID string, exportName string) (*core.MappingNode, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			if export, ok := instance.Exports[exportName]; ok {
				return export, nil
			}
		}
	}

	return nil, errors.New("export not found")
}

func (c *memoryExportsContainer) SaveAll(
	ctx context.Context,
	instanceID string,
	exports map[string]*core.MappingNode,
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
	export *core.MappingNode,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			instance.Exports[exportName] = export
		} else {
			return state.InstanceNotFoundError(instanceID)
		}
	}

	return nil
}

func (c *memoryExportsContainer) RemoveAll(ctx context.Context, instanceID string) (map[string]*core.MappingNode, error) {
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

func (c *memoryExportsContainer) Remove(ctx context.Context, instanceID string, exportName string) (*core.MappingNode, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			export, ok := instance.Exports[exportName]
			if ok {
				delete(instance.Exports, exportName)
				return export, nil
			}
		}
	}

	return nil, errors.New("export not found")
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
				return *child, nil
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

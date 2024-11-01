// An in-memory implementation of the StateContainer interface
// to be used for testing purposes.

package internal

import (
	"context"
	"errors"
	"sync"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

type MemoryStateContainer struct {
	instances map[string]*state.InstanceState
	mu        sync.RWMutex
}

func NewMemoryStateContainer() state.Container {
	return &MemoryStateContainer{
		instances: make(map[string]*state.InstanceState),
	}
}

func (c *MemoryStateContainer) GetInstance(ctx context.Context, instanceID string) (state.InstanceState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		return *instance, nil
	}

	return state.InstanceState{}, errors.New("instance not found")
}

func (c *MemoryStateContainer) SaveInstance(
	ctx context.Context,
	instanceState state.InstanceState,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.instances[instanceState.InstanceID] = &instanceState

	return nil
}

func (c *MemoryStateContainer) RemoveInstance(ctx context.Context, instanceID string) (state.InstanceState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	instance, ok := c.instances[instanceID]
	if !ok {
		return state.InstanceState{}, errors.New("instance not found")
	}

	delete(c.instances, instanceID)
	return *instance, nil
}

func (c *MemoryStateContainer) GetResource(ctx context.Context, instanceID string, resourceID string) (state.ResourceState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			if resourceState, ok := instance.Resources[resourceID]; ok {
				return *resourceState, nil
			}
		}
	}

	return state.ResourceState{}, nil
}

func (c *MemoryStateContainer) SaveResource(
	ctx context.Context,
	instanceID string,
	index int,
	resourceState state.ResourceState,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			instance.Resources[resourceState.ResourceID] = &resourceState
			resourceIDList, ok := instance.ResourceIDs[resourceState.ResourceName]
			if !ok {
				instance.ResourceIDs[resourceState.ResourceName] = []string{
					resourceState.ResourceID,
				}
			} else {
				instance.ResourceIDs[resourceState.ResourceName] = append(resourceIDList, resourceState.ResourceID)
			}
		} else {
			return errors.New("instance not found")
		}
	}

	return nil
}

func (c *MemoryStateContainer) RemoveResource(
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

	return state.ResourceState{}, errors.New("resource not found")
}

func (c *MemoryStateContainer) GetLink(ctx context.Context, instanceID string, linkID string) (state.LinkState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			if linkState, ok := instance.Links[linkID]; ok {
				return *linkState, nil
			}
		}
	}

	return state.LinkState{}, errors.New("link not found")
}

func (c *MemoryStateContainer) SaveLink(ctx context.Context, instanceID string, linkState state.LinkState) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			instance.Links[linkState.LinkID] = &linkState
		} else {
			return errors.New("instance not found")
		}
	}

	return nil
}

func (c *MemoryStateContainer) RemoveLink(ctx context.Context, instanceID string, linkID string) (state.LinkState, error) {
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

	return state.LinkState{}, errors.New("link not found")
}

func (c *MemoryStateContainer) GetMetadata(ctx context.Context, instanceID string) (map[string]*core.MappingNode, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			return instance.Metadata, nil
		}
	}

	return nil, errors.New("instance not found")
}

func (c *MemoryStateContainer) SaveMetadata(
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
			return errors.New("instance not found")
		}
	}

	return nil
}

func (c *MemoryStateContainer) RemoveMetadata(ctx context.Context, instanceID string) (map[string]*core.MappingNode, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			metadata := instance.Metadata
			instance.Metadata = nil
			return metadata, nil
		}
	}

	return nil, errors.New("instance not found")
}

func (c *MemoryStateContainer) GetExports(ctx context.Context, instanceID string) (map[string]*core.MappingNode, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			return instance.Exports, nil
		}
	}

	return nil, errors.New("instance not found")
}

func (c *MemoryStateContainer) GetExport(ctx context.Context, instanceID string, exportName string) (*core.MappingNode, error) {
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

func (c *MemoryStateContainer) SaveExports(
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
			return errors.New("instance not found")
		}
	}

	return nil
}

func (c *MemoryStateContainer) SaveExport(
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
			return errors.New("instance not found")
		}
	}

	return nil
}

func (c *MemoryStateContainer) RemoveExports(ctx context.Context, instanceID string) (map[string]*core.MappingNode, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			exports := instance.Exports
			instance.Exports = nil
			return exports, nil
		}
	}

	return nil, errors.New("instance not found")
}

func (c *MemoryStateContainer) RemoveExport(ctx context.Context, instanceID string, exportName string) (*core.MappingNode, error) {
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

func (c *MemoryStateContainer) GetChild(ctx context.Context, instanceID string, childName string) (state.InstanceState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			if child, ok := instance.ChildBlueprints[childName]; ok {
				return *child, nil
			}
		}
	}

	return state.InstanceState{}, errors.New("child not found")
}

func (c *MemoryStateContainer) SaveChild(
	ctx context.Context,
	instanceID string,
	childName string,
	childState state.InstanceState,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			instance.ChildBlueprints[childName] = &childState
		} else {
			return errors.New("instance not found")
		}
	}

	return nil
}

func (c *MemoryStateContainer) RemoveChild(ctx context.Context, instanceID string, childName string) (state.InstanceState, error) {
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

	return state.InstanceState{}, errors.New("child not found")
}

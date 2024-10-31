// An in-memory implementation of the StateContainer interface
// to be used for testing purposes.

package internal

import (
	"context"
	"sync"

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

func (c *MemoryStateContainer) GetResource(ctx context.Context, instanceID string, resourceID string) (*state.ResourceState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			if resource, ok := instance.Resources[resourceID]; ok {
				return resource, nil
			}
		}
	}

	return nil, nil
}

func (c *MemoryStateContainer) GetLink(ctx context.Context, instanceID string, linkID string) (*state.LinkState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			if link, ok := instance.Links[linkID]; ok {
				return link, nil
			}
		}
	}

	return nil, nil
}

func (c *MemoryStateContainer) GetInstance(ctx context.Context, instanceID string) (*state.InstanceState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		return instance, nil
	}

	return nil, nil
}

func (c *MemoryStateContainer) SaveInstance(
	ctx context.Context,
	instanceID string,
	instanceState state.InstanceState,
) (*state.InstanceState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.instances[instanceID] = &instanceState

	return &instanceState, nil
}

func (c *MemoryStateContainer) RemoveInstance(ctx context.Context, instanceID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.instances, instanceID)

	return nil
}

func (c *MemoryStateContainer) SaveResource(
	ctx context.Context,
	instanceID string,
	resourceID string,
	resourceState *state.ResourceState,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			instance.Resources[resourceID] = resourceState
		}
	}

	return nil
}

func (c *MemoryStateContainer) RemoveResource(
	ctx context.Context,
	instanceID string,
	resourceID string,
) (*state.ResourceState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance != nil {
			resource, ok := instance.Resources[resourceID]
			if ok {
				delete(instance.Resources, resourceID)
				return resource, nil
			}
		}
	}

	return nil, nil
}

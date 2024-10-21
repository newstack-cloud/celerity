// An in-memory implementation of the StateContainer interface
// to be used for testing purposes.

package internal

import (
	"context"
	"sync"

	"github.com/two-hundred/celerity/libs/blueprint/state"
)

type instanceStateWrapper struct {
	current   *state.InstanceState
	revisions map[string]*state.InstanceState
}

type MemoryStateContainer struct {
	instances map[string]*instanceStateWrapper
	mu        sync.RWMutex
}

func NewMemoryStateContainer() state.Container {
	return &MemoryStateContainer{
		instances: make(map[string]*instanceStateWrapper),
	}
}

func (c *MemoryStateContainer) GetResource(ctx context.Context, instanceID string, resourceID string) (*state.ResourceState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if instance.current != nil {
			if resource, ok := instance.current.Resources[resourceID]; ok {
				return resource, nil
			}
		}
	}

	return nil, nil
}

func (c *MemoryStateContainer) GetResourceForRevision(
	ctx context.Context,
	instanceID string,
	revisionID string,
	resourceID string,
) (*state.ResourceState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if revision, ok := instance.revisions[revisionID]; ok {
			if resource, ok := revision.Resources[resourceID]; ok {
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
		if instance.current != nil {
			if link, ok := instance.current.Links[linkID]; ok {
				return link, nil
			}
		}
	}

	return nil, nil
}

func (c *MemoryStateContainer) GetLinkForRevision(
	ctx context.Context,
	instanceID string,
	revisionID string,
	linkID string,
) (*state.LinkState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if revision, ok := instance.revisions[revisionID]; ok {
			if link, ok := revision.Links[linkID]; ok {
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
		return instance.current, nil
	}

	return nil, nil
}

func (c *MemoryStateContainer) GetInstanceRevision(
	ctx context.Context,
	instanceID string,
	revisionID string,
) (*state.InstanceState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if instance, ok := c.instances[instanceID]; ok {
		if revision, ok := instance.revisions[revisionID]; ok {
			return revision, nil
		}
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

	if instance, ok := c.instances[instanceID]; ok {
		instance.current = &instanceState
	} else {
		c.instances[instanceID] = &instanceStateWrapper{
			current: &instanceState,
		}
	}

	return &instanceState, nil
}

func (c *MemoryStateContainer) RemoveInstance(ctx context.Context, instanceID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.instances, instanceID)

	return nil
}

func (c *MemoryStateContainer) RemoveInstanceRevision(
	ctx context.Context,
	instanceID string,
	revisionID string,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		delete(instance.revisions, revisionID)
	}

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
		if instance.current != nil {
			instance.current.Resources[resourceID] = resourceState
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
		if instance.current != nil {
			resource, ok := instance.current.Resources[resourceID]
			if ok {
				delete(instance.current.Resources, resourceID)
				return resource, nil
			}
		}
	}

	return nil, nil
}

func (c *MemoryStateContainer) CleanupRevisions(ctx context.Context, instanceID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance, ok := c.instances[instanceID]; ok {
		instance.revisions = make(map[string]*state.InstanceState)
	}

	return nil
}

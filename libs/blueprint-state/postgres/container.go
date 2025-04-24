package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/two-hundred/celerity/libs/blueprint-state/manage"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

// StateContainer provides the postgres implementation of
// the blueprint `state.Container` interface
// along with methods to manage persistence for
// blueprint validation requests, events and change sets.
type StateContainer struct {
	instancesContainer  *instancesContainerImpl
	resourcesContainer  *resourcesContainerImpl
	linksContainer      *linksContainerImpl
	childrenContainer   *childrenContainerImpl
	metadataContainer   *metadataContainerImpl
	exportContainer     *exportContainerImpl
	validationContainer *validationContainerImpl
	changesetsContainer *changesetsContainerImpl
	eventsContainer     *eventsContainerImpl
}

// LoadStateContainer loads a new state container
// that uses postgres for persistence.
//
// The postgres connection pool must be configured appropriately
// in the calling application where the application will take care of making
// sure the connection pool is cleaned up when a command is finished or the application
// is shutting down.
func LoadStateContainer(
	ctx context.Context,
	connPool *pgxpool.Pool,
	logger core.Logger,
) (*StateContainer, error) {
	instancesContainer := &instancesContainerImpl{
		connPool: connPool,
	}

	container := &StateContainer{
		instancesContainer: instancesContainer,
		resourcesContainer: &resourcesContainerImpl{
			connPool: connPool,
		},
		linksContainer: &linksContainerImpl{
			connPool: connPool,
		},
		childrenContainer: &childrenContainerImpl{
			connPool:  connPool,
			instances: instancesContainer,
		},
		metadataContainer: &metadataContainerImpl{
			connPool: connPool,
		},
		exportContainer: &exportContainerImpl{
			connPool: connPool,
		},
		eventsContainer: &eventsContainerImpl{
			connPool: connPool,
			logger:   logger,
		},
		changesetsContainer: &changesetsContainerImpl{
			connPool: connPool,
			logger:   logger,
		},
		validationContainer: &validationContainerImpl{
			connPool: connPool,
			logger:   logger,
		},
	}

	return container, nil
}

func (c *StateContainer) Instances() state.InstancesContainer {
	return c.instancesContainer
}

func (c *StateContainer) Resources() state.ResourcesContainer {
	return c.resourcesContainer
}

func (c *StateContainer) Links() state.LinksContainer {
	return c.linksContainer
}

func (c *StateContainer) Children() state.ChildrenContainer {
	return c.childrenContainer
}

func (c *StateContainer) Metadata() state.MetadataContainer {
	return c.metadataContainer
}

func (c *StateContainer) Exports() state.ExportsContainer {
	return c.exportContainer
}

func (c *StateContainer) Validation() manage.Validation {
	return c.validationContainer
}

func (c *StateContainer) Changesets() manage.Changesets {
	return c.changesetsContainer
}

func (c *StateContainer) Events() manage.Events {
	return c.eventsContainer
}

package drift

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/changes"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

// Checker is an interface for behaviour
// that can be used to check if resources within
// a blueprint have drifted from the current state
// persisted with the blueprint framework.
// This is useful to detect situations where resources
// in an upstream provider (e.g. an AWS account) have been modified
// manually or by other means, and the blueprint state
// is no longer in sync with the actual state of the
// resources.
// A checker is only responsible for checking and persisting
// drift, the course of action to resolve the drift is
// left to the user.
type Checker interface {
	// CheckDrift checks the drift of all resources in the blueprint
	// with the given instance ID.
	// This will always check the drift with the upstream provider,
	// the state container can be used to retrieve the last known
	// drift state that was previously checked.
	// In most cases, this method will persist the results of the
	// drift check with the configured state container.
	// This returns a map of resource IDs to their drift state.
	CheckDrift(
		ctx context.Context,
		instanceID string,
		params core.BlueprintParams,
	) (map[string]*state.ResourceDriftState, error)
	// CheckResourceDrift checks the drift of a single resource
	// with the given instance ID and resource ID.
	// This will always check the drift with the upstream provider,
	// the state container can be used to retrieve the last known
	// drift state that was previously checked.
	// In most cases, this method will persist the results of the
	// drift check with the configured state container.
	CheckResourceDrift(
		ctx context.Context,
		instanceID string,
		resourceID string,
		params core.BlueprintParams,
	) (*state.ResourceDriftState, error)
}

type defaultChecker struct {
	stateContainer  state.Container
	providers       map[string]provider.Provider
	changeGenerator changes.ResourceChangeGenerator
}

// NewDefaultChecker creates a new instance
// of the default drift checker implementation.
func NewDefaultChecker(
	stateContainer state.Container,
	providers map[string]provider.Provider,
	changeGenerator changes.ResourceChangeGenerator,
) Checker {
	return &defaultChecker{
		stateContainer,
		providers,
		changeGenerator,
	}
}

func (c *defaultChecker) CheckDrift(
	ctx context.Context,
	instanceID string,
	params core.BlueprintParams,
) (map[string]*state.ResourceDriftState, error) {
	return nil, nil
}

func (c *defaultChecker) CheckResourceDrift(
	ctx context.Context,
	instanceID string,
	resourceID string,
	params core.BlueprintParams,
) (*state.ResourceDriftState, error) {
	return nil, nil
}

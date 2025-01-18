package container

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

// LinkDestroyer provides an interface for a service
// that destroys a link between two resources.
type LinkDestroyer interface {
	Destroy(
		ctx context.Context,
		element state.Element,
		instanceID string,
		deployCtx *DeployContext,
	)
}

// NewDefaultLinkDestroyer creates a new instance of the default
// implementation of the service that destroys a link between
// two resources.
func NewDefaultLinkDestroyer(
	linkDeployer LinkDeployer,
	linkRegistry provider.LinkRegistry,
	defaultRetryPolicy *provider.RetryPolicy,
) LinkDestroyer {
	return &defaultLinkDestroyer{
		linkDeployer:       linkDeployer,
		linkRegistry:       linkRegistry,
		defaultRetryPolicy: defaultRetryPolicy,
	}
}

type defaultLinkDestroyer struct {
	linkDeployer       LinkDeployer
	linkRegistry       provider.LinkRegistry
	defaultRetryPolicy *provider.RetryPolicy
}

func (d *defaultLinkDestroyer) Destroy(
	ctx context.Context,
	element state.Element,
	instanceID string,
	deployCtx *DeployContext,
) {
	linkState := getLinkStateByName(
		deployCtx.InstanceStateSnapshot,
		element.LogicalName(),
	)
	if linkState == nil {
		deployCtx.Channels.ErrChan <- errLinkNotFoundInState(
			element.LogicalName(),
			instanceID,
		)
		return
	}

	linkImplementation, err := d.getProviderLinkImplementation(
		ctx,
		element.LogicalName(),
		deployCtx.InstanceStateSnapshot,
	)
	if err != nil {
		deployCtx.Channels.ErrChan <- err
		return
	}

	retryPolicy, err := getLinkRetryPolicy(
		ctx,
		element.LogicalName(),
		deployCtx.InstanceStateSnapshot,
		d.linkRegistry,
		d.defaultRetryPolicy,
	)
	if err != nil {
		deployCtx.Channels.ErrChan <- err
		return
	}

	_, err = d.linkDeployer.Deploy(
		ctx,
		element,
		instanceID,
		provider.LinkUpdateTypeDestroy,
		linkImplementation,
		deployCtx,
		retryPolicy,
	)
	if err != nil {
		deployCtx.Channels.ErrChan <- err
	}
}

func (d *defaultLinkDestroyer) getProviderLinkImplementation(
	ctx context.Context,
	linkName string,
	currentState *state.InstanceState,
) (provider.Link, error) {

	resourceTypeA, resourceTypeB, err := getResourceTypesForLink(linkName, currentState)
	if err != nil {
		return nil, err
	}

	return d.linkRegistry.Link(ctx, resourceTypeA, resourceTypeB)
}

package container

import (
	"context"
	"time"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
)

// ResourceDeployer provides an interface for a service that deploys
// a resource as a part of the deployment process for a blueprint instance.
type ResourceDeployer interface {
	Deploy(
		ctx context.Context,
		instanceID string,
		chainLinkNode *links.ChainLinkNode,
		changes *BlueprintChanges,
		deployCtx *DeployContext,
	)
}

// NewDefaultResourceDeployer creates a new instance of the default
// implementation of the service that deploys a resource as a part of
// the deployment process for a blueprint instance.
func NewDefaultResourceDeployer(
	clock core.Clock,
	idGenerator core.IDGenerator,
	defaultRetryPolicy *provider.RetryPolicy,
	substitutionResolver subengine.SubstitutionResolver,
	resourceCache *core.Cache[*provider.ResolvedResource],
) ResourceDeployer {
	return &defaultResourceDeployer{
		clock:                clock,
		idGenerator:          idGenerator,
		defaultRetryPolicy:   defaultRetryPolicy,
		substitutionResolver: substitutionResolver,
		resourceCache:        resourceCache,
	}
}

type defaultResourceDeployer struct {
	clock                core.Clock
	idGenerator          core.IDGenerator
	defaultRetryPolicy   *provider.RetryPolicy
	substitutionResolver subengine.SubstitutionResolver
	resourceCache        *core.Cache[*provider.ResolvedResource]
}

func (d *defaultResourceDeployer) Deploy(
	ctx context.Context,
	instanceID string,
	chainLinkNode *links.ChainLinkNode,
	changes *BlueprintChanges,
	deployCtx *DeployContext,
) {
	resourceChangeInfo := getResourceChangeInfo(chainLinkNode.ResourceName, changes)
	if resourceChangeInfo == nil {
		deployCtx.Channels.ErrChan <- errMissingResourceChanges(
			chainLinkNode.ResourceName,
		)
		return
	}
	partiallyResolvedResource := getResolvedResourceFromChanges(resourceChangeInfo.changes)
	if partiallyResolvedResource == nil {
		deployCtx.Channels.ErrChan <- errMissingPartiallyResolvedResource(
			chainLinkNode.ResourceName,
		)
		return
	}
	resolvedResource, err := d.resolveResourceForDeployment(
		ctx,
		partiallyResolvedResource,
		chainLinkNode,
		changes.ResolveOnDeploy,
	)
	if err != nil {
		deployCtx.Channels.ErrChan <- err
		return
	}

	resourceID, err := d.getResourceID(resourceChangeInfo.changes)
	if err != nil {
		deployCtx.Channels.ErrChan <- err
		return
	}

	resourceImplementation, err := getProviderResourceImplementation(
		ctx,
		chainLinkNode.ResourceName,
		resolvedResource.Type.Value,
		deployCtx.ResourceProviders,
	)
	if err != nil {
		deployCtx.Channels.ErrChan <- err
		return
	}

	policy, err := getRetryPolicy(
		ctx,
		deployCtx.ResourceProviders,
		chainLinkNode.ResourceName,
		d.defaultRetryPolicy,
	)
	if err != nil {
		deployCtx.Channels.ErrChan <- err
		return
	}

	// The resource state is made available in a change set at the time
	// changes were staged, this is primarily to provide a convenient way
	// to surface the current state to users during the "planning" phase.
	// As there can be a significant delay between change staging and deployment,
	// we'll replace the current state in the change set with the latest snapshot
	// of the resource state.
	resourceState := getResourceStateByName(
		deployCtx.InstanceStateSnapshot,
		chainLinkNode.ResourceName,
	)

	err = d.deployResource(
		ctx,
		&resourceDeployInfo{
			instanceID:   instanceID,
			resourceID:   resourceID,
			resourceName: chainLinkNode.ResourceName,
			resourceImpl: resourceImplementation,
			changes: prepareResourceChangesForDeployment(
				resourceChangeInfo.changes,
				resolvedResource,
				resourceState,
				resourceID,
				instanceID,
			),
			isNew: resourceChangeInfo.isNew,
		},
		deployCtx,
		createRetryInfo(policy),
	)
	if err != nil {
		deployCtx.Channels.ErrChan <- err
	}
}

func (d *defaultResourceDeployer) deployResource(
	ctx context.Context,
	resourceInfo *resourceDeployInfo,
	deployCtx *DeployContext,
	resourceRetryInfo *retryInfo,
) error {
	resourceDeploymentStartTime := d.clock.Now()
	deployCtx.Channels.ResourceUpdateChan <- ResourceDeployUpdateMessage{
		InstanceID:   resourceInfo.instanceID,
		ResourceID:   resourceInfo.resourceID,
		ResourceName: resourceInfo.resourceName,
		Group:        deployCtx.CurrentGroupIndex,
		Status: determineResourceDeployingStatus(
			deployCtx.Rollback,
			resourceInfo.isNew,
		),
		PreciseStatus: determinePreciseResourceDeployingStatus(
			deployCtx.Rollback,
			resourceInfo.isNew,
		),
		UpdateTimestamp: d.clock.Now().Unix(),
		Attempt:         resourceRetryInfo.attempt,
	}

	output, err := resourceInfo.resourceImpl.Deploy(
		ctx,
		&provider.ResourceDeployInput{
			Changes: resourceInfo.changes,
			Params:  deployCtx.ParamOverrides,
		},
	)
	if err != nil {
		if provider.IsRetryableError(err) {
			retryErr := err.(*provider.RetryableError)
			return d.handleDeployResourceRetry(
				ctx,
				resourceInfo,
				resourceRetryInfo,
				resourceDeploymentStartTime,
				[]string{retryErr.ChildError.Error()},
				deployCtx,
			)
		}

		if provider.IsResourceDeployError(err) {
			resourceDeployError := err.(*provider.ResourceDeployError)
			return d.handleDeployResourceTerminalFailure(
				resourceInfo,
				resourceRetryInfo,
				resourceDeploymentStartTime,
				resourceDeployError.FailureReasons,
				deployCtx,
			)
		}

		// For errors that are not wrapped in a provider error, the error is assumed
		// to be fatal and the deployment process will be stopped without reporting
		// a failure status.
		// It is really important that adequate guidance is provided for provider developers
		// to ensure that all errors are wrapped in the appropriate provider error.
		return err
	}

	deployCtx.State.SetResourceSpecState(resourceInfo.resourceName, output.SpecState)
	// At this point, we mark the resource as "config complete", the listener
	// should be able to determine if the resource is stable and ready for dependent
	// resources to be deployed.
	// Once the resource is stable, a status update will be sent with the appropriate
	// "deployed" status.
	deployCtx.Channels.ResourceUpdateChan <- ResourceDeployUpdateMessage{
		InstanceID:   resourceInfo.instanceID,
		ResourceID:   resourceInfo.resourceID,
		ResourceName: resourceInfo.resourceName,
		Group:        deployCtx.CurrentGroupIndex,
		Status: determineResourceConfigCompleteStatus(
			deployCtx.Rollback,
			resourceInfo.isNew,
		),
		PreciseStatus: determinePreciseResourceConfigCompleteStatus(
			deployCtx.Rollback,
			resourceInfo.isNew,
		),
		UpdateTimestamp: d.clock.Now().Unix(),
		Attempt:         resourceRetryInfo.attempt,
		Durations: determineResourceDeployConfigCompleteDurations(
			resourceRetryInfo,
			d.clock.Since(resourceDeploymentStartTime),
		),
	}

	return nil
}

func (d *defaultResourceDeployer) handleDeployResourceRetry(
	ctx context.Context,
	resourceInfo *resourceDeployInfo,
	resourceRetryInfo *retryInfo,
	resourceDeploymentStartTime time.Time,
	failureReasons []string,
	deployCtx *DeployContext,
) error {
	currentAttemptDuration := d.clock.Since(resourceDeploymentStartTime)
	nextRetryInfo := addRetryAttempt(resourceRetryInfo, currentAttemptDuration)
	deployCtx.Channels.ResourceUpdateChan <- ResourceDeployUpdateMessage{
		InstanceID:   resourceInfo.instanceID,
		ResourceID:   resourceInfo.resourceID,
		ResourceName: resourceInfo.resourceName,
		Group:        deployCtx.CurrentGroupIndex,
		Status: determineResourceDeployFailedStatus(
			deployCtx.Rollback,
			resourceInfo.isNew,
		),
		PreciseStatus: determinePreciseResourceDeployFailedStatus(
			deployCtx.Rollback,
			resourceInfo.isNew,
		),
		FailureReasons:  failureReasons,
		CanRetry:        !nextRetryInfo.exceededMaxRetries,
		UpdateTimestamp: d.clock.Now().Unix(),
		// Attempt durations will be accumulated and sent in the status updates
		// for each subsequent retry.
		// Total duration will be calculated if retry limit is exceeded.
		Durations: determineResourceRetryFailureDurations(
			nextRetryInfo,
		),
	}

	if !nextRetryInfo.exceededMaxRetries {
		waitTimeMs := provider.CalculateRetryWaitTimeMS(nextRetryInfo.policy, nextRetryInfo.attempt)
		time.Sleep(time.Duration(waitTimeMs) * time.Millisecond)
		return d.deployResource(
			ctx,
			resourceInfo,
			deployCtx,
			nextRetryInfo,
		)
	}

	return nil
}

func (d *defaultResourceDeployer) handleDeployResourceTerminalFailure(
	resourceInfo *resourceDeployInfo,
	resourceRetryInfo *retryInfo,
	resourceDeploymentStartTime time.Time,
	failureReasons []string,
	deployCtx *DeployContext,
) error {
	currentAttemptDuration := d.clock.Since(resourceDeploymentStartTime)
	deployCtx.Channels.ResourceUpdateChan <- ResourceDeployUpdateMessage{
		InstanceID:   resourceInfo.instanceID,
		ResourceID:   resourceInfo.resourceID,
		ResourceName: resourceInfo.resourceName,
		Group:        deployCtx.CurrentGroupIndex,
		Status: determineResourceDeployFailedStatus(
			deployCtx.Rollback,
			resourceInfo.isNew,
		),
		PreciseStatus: determinePreciseResourceDeployFailedStatus(
			deployCtx.Rollback,
			resourceInfo.isNew,
		),
		FailureReasons:  failureReasons,
		Attempt:         resourceRetryInfo.attempt,
		CanRetry:        false,
		UpdateTimestamp: d.clock.Now().Unix(),
		Durations: determineResourceDeployFinishedDurations(
			resourceRetryInfo,
			currentAttemptDuration,
			/* configCompleteDuration */ nil,
		),
	}

	return nil
}

func (d *defaultResourceDeployer) resolveResourceForDeployment(
	ctx context.Context,
	partiallyResolvedResource *provider.ResolvedResource,
	node *links.ChainLinkNode,
	resolveOnDeploy []string,
) (*provider.ResolvedResource, error) {
	if !resourceHasFieldsToResolve(node.ResourceName, resolveOnDeploy) {
		return partiallyResolvedResource, nil
	}

	resolveResourceResult, err := d.substitutionResolver.ResolveInResource(
		ctx,
		node.ResourceName,
		node.Resource,
		&subengine.ResolveResourceTargetInfo{
			ResolveFor:        subengine.ResolveForDeployment,
			PartiallyResolved: partiallyResolvedResource,
		},
	)
	if err != nil {
		return nil, err
	}

	// Cache the resolved resource so that it can be used in resolving other elements
	// that reference fields in the current resource.
	d.resourceCache.Set(
		node.ResourceName,
		resolveResourceResult.ResolvedResource,
	)

	return resolveResourceResult.ResolvedResource, nil
}

func (d *defaultResourceDeployer) getResourceID(changes *provider.Changes) (string, error) {
	if changes.AppliedResourceInfo.ResourceID == "" {
		return d.idGenerator.GenerateID()
	}

	return changes.AppliedResourceInfo.ResourceID, nil
}

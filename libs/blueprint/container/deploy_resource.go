package container

import (
	"context"
	"time"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
)

const (
	resourceStabilisingTimeoutFailureMessage = "Resource failed to stabilise within the configured timeout"
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

// ResourceSubstitutionResolver provides an interface for a service that
// is responsible for resolving substitutions in a resource definition.
type ResourceSubstitutionResolver interface {
	// ResolveInResource resolves substitutions in a resource.
	ResolveInResource(
		ctx context.Context,
		resourceName string,
		resource *schema.Resource,
		resolveTargetInfo *subengine.ResolveResourceTargetInfo,
	) (*subengine.ResolveInResourceResult, error)
}

// NewDefaultResourceDeployer creates a new instance of the default
// implementation of the service that deploys a resource as a part of
// the deployment process for a blueprint instance.
func NewDefaultResourceDeployer(
	clock core.Clock,
	idGenerator core.IDGenerator,
	defaultRetryPolicy *provider.RetryPolicy,
	stabilityPollingConfig *ResourceStabilityPollingConfig,
	substitutionResolver ResourceSubstitutionResolver,
	resourceCache *core.Cache[*provider.ResolvedResource],
	stateContainer state.Container,
) ResourceDeployer {
	return &defaultResourceDeployer{
		clock:                  clock,
		idGenerator:            idGenerator,
		defaultRetryPolicy:     defaultRetryPolicy,
		substitutionResolver:   substitutionResolver,
		stabilityPollingConfig: stabilityPollingConfig,
		resourceCache:          resourceCache,
		stateContainer:         stateContainer,
	}
}

type defaultResourceDeployer struct {
	clock                  core.Clock
	idGenerator            core.IDGenerator
	defaultRetryPolicy     *provider.RetryPolicy
	stabilityPollingConfig *ResourceStabilityPollingConfig
	substitutionResolver   ResourceSubstitutionResolver
	resourceCache          *core.Cache[*provider.ResolvedResource]
	stateContainer         state.Container
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

	if resourceChangeInfo.isNew {
		resources := d.stateContainer.Resources()
		err := resources.Save(ctx, instanceID, state.ResourceState{
			ResourceID:    resourceID,
			ResourceName:  chainLinkNode.ResourceName,
			ResourceType:  resolvedResource.Type.Value,
			Status:        core.ResourceStatusUnknown,
			PreciseStatus: core.PreciseResourceStatusUnknown,
		})
		if err != nil {
			deployCtx.Channels.ErrChan <- err
			return
		}
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
		resolvedResource.Type.Value,
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
	resourceType string,
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

	providerNamespace := provider.ExtractProviderFromItemType(resourceType)
	output, err := resourceInfo.resourceImpl.Deploy(
		ctx,
		&provider.ResourceDeployInput{
			InstanceID: resourceInfo.instanceID,
			ResourceID: resourceInfo.resourceID,
			Changes:    resourceInfo.changes,
			ProviderContext: provider.NewProviderContextFromParams(
				providerNamespace,
				deployCtx.ParamOverrides,
			),
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

	resolvedResource := getResolvedResourceFromChanges(resourceInfo.changes)
	mergedSpecState, err := MergeResourceSpec(
		resolvedResource,
		resourceInfo.resourceName,
		output.ComputedFieldValues,
		resourceInfo.changes.ComputedFields,
	)
	if err != nil {
		return err
	}

	deployCtx.State.SetResourceData(
		resourceInfo.resourceName,
		&CollectedResourceData{
			Spec: mergedSpecState,
			Metadata: resolvedMetadataToState(
				extractResolvedMetadataFromResourceInfo(resourceInfo),
			),
		},
	)
	// At this point, we mark the resource as "config complete", a separate coroutine
	// is invoked asynchronously to poll the resource for stability.
	// Once the resource is stable, a status update will be sent with the appropriate
	// "deployed" status.
	configCompleteMsg := ResourceDeployUpdateMessage{
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
	deployCtx.Channels.ResourceUpdateChan <- configCompleteMsg
	deployCtx.State.SetResourceDurationInfo(
		resourceInfo.resourceName,
		configCompleteMsg.Durations,
	)

	go d.pollForResourceStability(
		ctx,
		resourceInfo,
		resourceRetryInfo,
		deployCtx,
	)

	return nil
}

func (d *defaultResourceDeployer) pollForResourceStability(
	ctx context.Context,
	resourceInfo *resourceDeployInfo,
	resourceRetryInfo *retryInfo,
	deployCtx *DeployContext,
) {
	pollingStabilisationStartTime := d.clock.Now()

	ctxWithPollingTimeout, cancel := context.WithTimeout(
		ctx,
		d.stabilityPollingConfig.PollingTimeout,
	)
	defer cancel()

	for {
		select {
		case <-ctxWithPollingTimeout.Done():
			deployCtx.Channels.ResourceUpdateChan <- d.createResourceStabiliseTimeoutMessage(
				resourceInfo,
				resourceRetryInfo,
				pollingStabilisationStartTime,
				deployCtx,
			)
			return
		case <-time.After(d.stabilityPollingConfig.PollingInterval):
			resourceData := deployCtx.State.GetResourceData(resourceInfo.resourceName)
			resolvedResource := getResolvedResourceFromChanges(resourceInfo.changes)
			providerNamespace := provider.ExtractProviderFromItemType(
				getResourceTypeFromResolved(resolvedResource),
			)
			output, err := resourceInfo.resourceImpl.HasStabilised(
				ctxWithPollingTimeout,
				&provider.ResourceHasStabilisedInput{
					InstanceID:       resourceInfo.instanceID,
					ResourceID:       resourceInfo.resourceID,
					ResourceSpec:     resourceData.Spec,
					ResourceMetadata: resourceData.Metadata,
					ProviderContext: provider.NewProviderContextFromParams(
						providerNamespace,
						deployCtx.ParamOverrides,
					),
				},
			)
			if err != nil {
				deployCtx.Channels.ErrChan <- err
				return
			}

			if output.Stabilised {
				deployCtx.Channels.ResourceUpdateChan <- d.createResourceStabilisedMessage(
					resourceInfo,
					resourceRetryInfo,
					pollingStabilisationStartTime,
					deployCtx,
				)
				return
			}
		}
	}
}

func (d *defaultResourceDeployer) createResourceStabiliseTimeoutMessage(
	resourceInfo *resourceDeployInfo,
	resourceRetryInfo *retryInfo,
	pollingStabilisationStartTime time.Time,
	deployCtx *DeployContext,
) ResourceDeployUpdateMessage {
	configCompleteDurationInfo := deployCtx.State.GetResourceDurationInfo(
		resourceInfo.resourceName,
	)

	return ResourceDeployUpdateMessage{
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
		FailureReasons:  []string{resourceStabilisingTimeoutFailureMessage},
		Attempt:         resourceRetryInfo.attempt,
		CanRetry:        false,
		UpdateTimestamp: d.clock.Now().Unix(),
		Durations: addTotalToResourceCompletionDurations(
			configCompleteDurationInfo,
			d.clock.Since(pollingStabilisationStartTime),
		),
	}
}

func (d *defaultResourceDeployer) createResourceStabilisedMessage(
	resourceInfo *resourceDeployInfo,
	resourceRetryInfo *retryInfo,
	pollingStabilisationStartTime time.Time,
	deployCtx *DeployContext,
) ResourceDeployUpdateMessage {
	configCompleteDurationInfo := deployCtx.State.GetResourceDurationInfo(
		resourceInfo.resourceName,
	)

	return ResourceDeployUpdateMessage{
		InstanceID:   resourceInfo.instanceID,
		ResourceID:   resourceInfo.resourceID,
		ResourceName: resourceInfo.resourceName,
		Group:        deployCtx.CurrentGroupIndex,
		Status: determineResourceDeployedStatus(
			deployCtx.Rollback,
			resourceInfo.isNew,
		),
		PreciseStatus: determinePreciseResourceDeployedStatus(
			deployCtx.Rollback,
			resourceInfo.isNew,
		),
		Attempt:         resourceRetryInfo.attempt,
		CanRetry:        false,
		UpdateTimestamp: d.clock.Now().Unix(),
		Durations: addTotalToResourceCompletionDurations(
			configCompleteDurationInfo,
			d.clock.Since(pollingStabilisationStartTime),
		),
	}
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
		Attempt:         resourceRetryInfo.attempt,
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
		resolvedResource := getResolvedResourceFromChanges(resourceInfo.changes)
		resourceType := getResourceTypeFromResolved(resolvedResource)
		return d.deployResource(
			ctx,
			resourceInfo,
			resourceType,
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

// ResourceStabilityPollingConfig represents the configuration for
// polling resources for stability.
type ResourceStabilityPollingConfig struct {
	// PollingInterval is the interval at which the resource will be polled
	// for stability.
	PollingInterval time.Duration
	// PollingTimeout is the maximum amount of time that the resource will be
	// polled for stability.
	PollingTimeout time.Duration
}

// DefaultResourceStabilityPollingConfig is a reasonable default configuration
// for polling resources for stability.
var DefaultResourceStabilityPollingConfig = &ResourceStabilityPollingConfig{
	PollingInterval: 5 * time.Second,
	PollingTimeout:  30 * time.Minute,
}

func (d *defaultResourceDeployer) getResourceID(changes *provider.Changes) (string, error) {
	if changes.AppliedResourceInfo.ResourceID == "" {
		return d.idGenerator.GenerateID()
	}

	return changes.AppliedResourceInfo.ResourceID, nil
}

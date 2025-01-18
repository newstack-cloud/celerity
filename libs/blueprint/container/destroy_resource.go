package container

import (
	"context"
	"time"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

// ResourceDestroyer provides an interface for a service that destroys a resource
// as a part of the deployment process for a blueprint instance.
type ResourceDestroyer interface {
	Destroy(
		ctx context.Context,
		resourceElement state.Element,
		instanceID string,
		deployCtx *DeployContext,
	)
}

// NewDefaultResourceDestroyer creates a new instance of the default implementation
// of the service that destroys a resource as a part of the deployment process for a blueprint instance.
func NewDefaultResourceDestroyer(
	clock core.Clock,
	defaultRetryPolicy *provider.RetryPolicy,
) ResourceDestroyer {
	return &defaultResourceDestroyer{
		clock:              clock,
		defaultRetryPolicy: defaultRetryPolicy,
	}
}

type defaultResourceDestroyer struct {
	clock              core.Clock
	defaultRetryPolicy *provider.RetryPolicy
}

func (d *defaultResourceDestroyer) Destroy(
	ctx context.Context,
	resourceElement state.Element,
	instanceID string,
	deployCtx *DeployContext,
) {
	resourceState := getResourceStateByName(
		deployCtx.InstanceStateSnapshot,
		resourceElement.LogicalName(),
	)
	if resourceState == nil {
		deployCtx.Channels.ErrChan <- errResourceNotFoundInState(
			resourceElement.LogicalName(),
			instanceID,
		)
		return
	}

	resourceImplementation, err := getProviderResourceImplementation(
		ctx,
		resourceElement.LogicalName(),
		resourceState.ResourceType,
		deployCtx.ResourceProviders,
	)
	if err != nil {
		deployCtx.Channels.ErrChan <- err
		return
	}

	policy, err := getRetryPolicy(
		ctx,
		deployCtx.ResourceProviders,
		resourceElement.LogicalName(),
		d.defaultRetryPolicy,
	)
	if err != nil {
		deployCtx.Channels.ErrChan <- err
		return
	}

	err = d.destroyResource(
		ctx,
		&deploymentElementInfo{
			element:    resourceElement,
			instanceID: instanceID,
		},
		resourceImplementation,
		deployCtx,
		createRetryInfo(policy),
	)
	if err != nil {
		deployCtx.Channels.ErrChan <- err
	}
}

func (d *defaultResourceDestroyer) destroyResource(
	ctx context.Context,
	resourceInfo *deploymentElementInfo,
	resourceImplementation provider.Resource,
	deployCtx *DeployContext,
	resourceRetryInfo *retryInfo,
) error {
	resourceRemovalStartTime := d.clock.Now()
	deployCtx.Channels.ResourceUpdateChan <- ResourceDeployUpdateMessage{
		InstanceID:      resourceInfo.instanceID,
		ResourceID:      resourceInfo.element.ID(),
		ResourceName:    resourceInfo.element.LogicalName(),
		Group:           deployCtx.CurrentGroupIndex,
		Status:          determineResourceDestroyingStatus(deployCtx.Rollback),
		PreciseStatus:   determinePreciseResourceDestroyingStatus(deployCtx.Rollback),
		UpdateTimestamp: d.clock.Now().Unix(),
		Attempt:         resourceRetryInfo.attempt,
	}

	resourceState := getResourceStateByName(
		deployCtx.InstanceStateSnapshot,
		resourceInfo.element.LogicalName(),
	)
	providerNamespace := provider.ExtractProviderFromItemType(resourceState.ResourceType)
	err := resourceImplementation.Destroy(ctx, &provider.ResourceDestroyInput{
		InstanceID:    resourceInfo.instanceID,
		ResourceID:    resourceInfo.element.ID(),
		ResourceState: resourceState,
		ProviderContext: provider.NewProviderContextFromParams(
			providerNamespace,
			deployCtx.ParamOverrides,
		),
	})
	if err != nil {
		if provider.IsRetryableError(err) {
			retryErr := err.(*provider.RetryableError)
			return d.handleDestroyResourceRetry(
				ctx,
				resourceInfo,
				resourceImplementation,
				resourceRetryInfo,
				resourceRemovalStartTime,
				[]string{retryErr.ChildError.Error()},
				deployCtx,
			)
		}

		if provider.IsResourceDestroyError(err) {
			resourceDestroyErr := err.(*provider.ResourceDestroyError)
			return d.handleDestroyResourceTerminalFailure(
				resourceInfo,
				resourceRetryInfo,
				resourceRemovalStartTime,
				resourceDestroyErr.FailureReasons,
				deployCtx,
			)
		}

		// For errors that are not wrapped in a provider error, the error is assumed to be fatal
		// and the deployment process will be stopped without reporting a failure state.
		// It is really important that adequate guidance is provided for provider developers
		// to ensure that all errors are wrapped in the appropriate provider error.
		return err
	}

	deployCtx.Channels.ResourceUpdateChan <- ResourceDeployUpdateMessage{
		InstanceID:      resourceInfo.instanceID,
		ResourceID:      resourceInfo.element.ID(),
		ResourceName:    resourceInfo.element.LogicalName(),
		Group:           deployCtx.CurrentGroupIndex,
		Status:          determineResourceDestroyedStatus(deployCtx.Rollback),
		PreciseStatus:   determinePreciseResourceDestroyedStatus(deployCtx.Rollback),
		UpdateTimestamp: d.clock.Now().Unix(),
		Attempt:         resourceRetryInfo.attempt,
		Durations: determineResourceDeployFinishedDurations(
			resourceRetryInfo,
			d.clock.Since(resourceRemovalStartTime),
			/* configCompleteDuration */ nil,
		),
	}

	return nil
}

func (d *defaultResourceDestroyer) handleDestroyResourceRetry(
	ctx context.Context,
	resourceInfo *deploymentElementInfo,
	resourceImplementation provider.Resource,
	resourceRetryInfo *retryInfo,
	resourceRemovalStartTime time.Time,
	failureReasons []string,
	deployCtx *DeployContext,
) error {
	currentAttemptDuration := d.clock.Since(resourceRemovalStartTime)
	nextRetryInfo := addRetryAttempt(resourceRetryInfo, currentAttemptDuration)
	deployCtx.Channels.ResourceUpdateChan <- ResourceDeployUpdateMessage{
		InstanceID:      resourceInfo.instanceID,
		ResourceID:      resourceInfo.element.ID(),
		ResourceName:    resourceInfo.element.LogicalName(),
		Group:           deployCtx.CurrentGroupIndex,
		Status:          determineResourceDestroyFailedStatus(deployCtx.Rollback),
		PreciseStatus:   determinePreciseResourceDestroyFailedStatus(deployCtx.Rollback),
		FailureReasons:  failureReasons,
		Attempt:         resourceRetryInfo.attempt,
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
		waitTimeMS := provider.CalculateRetryWaitTimeMS(nextRetryInfo.policy, nextRetryInfo.attempt)
		time.Sleep(time.Duration(waitTimeMS) * time.Millisecond)
		return d.destroyResource(
			ctx,
			resourceInfo,
			resourceImplementation,
			deployCtx,
			nextRetryInfo,
		)
	}

	return nil
}

func (d *defaultResourceDestroyer) handleDestroyResourceTerminalFailure(
	resourceInfo *deploymentElementInfo,
	resourceRetryInfo *retryInfo,
	resourceRemovalStartTime time.Time,
	failureReasons []string,
	deployCtx *DeployContext,
) error {
	currentAttemptDuration := d.clock.Since(resourceRemovalStartTime)
	deployCtx.Channels.ResourceUpdateChan <- ResourceDeployUpdateMessage{
		InstanceID:      resourceInfo.instanceID,
		ResourceID:      resourceInfo.element.ID(),
		ResourceName:    resourceInfo.element.LogicalName(),
		Group:           deployCtx.CurrentGroupIndex,
		Status:          determineResourceDestroyFailedStatus(deployCtx.Rollback),
		PreciseStatus:   determinePreciseResourceDestroyFailedStatus(deployCtx.Rollback),
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

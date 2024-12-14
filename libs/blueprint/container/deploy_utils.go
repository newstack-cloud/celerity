package container

import (
	"time"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

func determineResourceDestroyingStatus(rollingBack bool) core.ResourceStatus {
	if rollingBack {
		return core.ResourceStatusRollingBack
	}

	return core.ResourceStatusDestroying
}

func determinePreciseResourceDestroyingStatus(rollingBack bool) core.PreciseResourceStatus {
	if rollingBack {
		// In the context of rolling back, destroying a resource is to roll back
		// the creation of the resource.
		return core.PreciseResourceStatusCreateRollingBack
	}

	return core.PreciseResourceStatusDestroying
}

func determineResourceDestroyFailedStatus(rollingBack bool) core.ResourceStatus {
	if rollingBack {
		return core.ResourceStatusRollbackFailed
	}

	return core.ResourceStatusDestroyFailed
}

func determinePreciseResourceDestroyFailedStatus(rollingBack bool) core.PreciseResourceStatus {
	if rollingBack {
		// In the context of rolling back, destroying a resource is to roll back
		// the creation of the resource.
		return core.PreciseResourceStatusCreateRollbackFailed
	}

	return core.PreciseResourceStatusDestroyFailed
}

func determineResourceDestroyedStatus(rollingBack bool) core.ResourceStatus {
	if rollingBack {
		return core.ResourceStatusRollbackComplete
	}

	return core.ResourceStatusDestroyed
}

func determinePreciseResourceDestroyedStatus(rollingBack bool) core.PreciseResourceStatus {
	if rollingBack {
		// In the context of rolling back, destroying a resource is to roll back
		// the creation of the resource.
		return core.PreciseResourceStatusCreateRollbackComplete
	}

	return core.PreciseResourceStatusDestroyed
}

func determineResourceRetryFailureDurations(
	currentRetryInfo *retryInfo,
) *state.ResourceCompletionDurations {
	if currentRetryInfo.exceededMaxRetries {
		totalDuration := core.Sum(currentRetryInfo.attemptDurations)
		return &state.ResourceCompletionDurations{
			TotalDuration:    &totalDuration,
			AttemptDurations: currentRetryInfo.attemptDurations,
		}
	}

	return &state.ResourceCompletionDurations{
		AttemptDurations: currentRetryInfo.attemptDurations,
	}
}

func determineResourceDestroyFinishedDurations(
	currentRetryInfo *retryInfo,
	currentAttemptDuration time.Duration,
) *state.ResourceCompletionDurations {
	updatedAttemptDurations := append(
		currentRetryInfo.attemptDurations,
		core.FractionalMilliseconds(currentAttemptDuration),
	)
	totalDuration := core.Sum(updatedAttemptDurations)
	return &state.ResourceCompletionDurations{
		TotalDuration:    &totalDuration,
		AttemptDurations: updatedAttemptDurations,
	}
}

func addRetryAttempt(retryInfoToUpdate *retryInfo, currentAttemptDuration time.Duration) *retryInfo {
	nextAttempt := retryInfoToUpdate.attempt + 1
	return &retryInfo{
		policy:  retryInfoToUpdate.policy,
		attempt: nextAttempt,
		attemptDurations: append(
			retryInfoToUpdate.attemptDurations,
			core.FractionalMilliseconds(currentAttemptDuration),
		),
		exceededMaxRetries: nextAttempt > retryInfoToUpdate.policy.MaxRetries,
	}
}

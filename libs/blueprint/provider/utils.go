package provider

import (
	"math"
	"math/rand/v2"
	"strings"
)

// ExtractProviderFromItemType extracts the provider namespace from a resource type
// or data source type.
func ExtractProviderFromItemType(itemType string) string {
	parts := strings.Split(itemType, "/")
	if len(parts) == 0 {
		return ""
	}

	return parts[0]
}

// CalculateRetryWaitTimeMS calculates the wait time in milliseconds between retries
// based on a provided retry policy and current retry attempt.
func CalculateRetryWaitTimeMS(
	retryPolicy *RetryPolicy,
	currentRetryAttempt int,
) int {
	// Interval is configured in seconds, convert to milliseconds
	// to allow for millisecond precision for fractional backoff rates.
	intervalMS := float64(retryPolicy.FirstRetryDelay * 1000)
	computedWaitTimeMS := intervalMS * math.Pow(
		retryPolicy.BackoffFactor,
		float64(currentRetryAttempt-1),
	)

	if retryPolicy.MaxDelay != -1 {
		computedWaitTimeMS = math.Min(
			computedWaitTimeMS,
			float64(retryPolicy.MaxDelay*1000),
		)
	}

	if retryPolicy.Jitter {
		computedWaitTimeMS = rand.Float64() * computedWaitTimeMS
	}

	return int(math.Trunc(computedWaitTimeMS))
}

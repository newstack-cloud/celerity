package provider

import (
	"math"
	"math/rand/v2"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/core"
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

type providerCtxFromParams struct {
	providerNamespace string
	blueprintParams   core.BlueprintParams
}

// NewProviderContextFromParams creates a new provider context
// from a set of blueprint parameters for the current environment.
// The provider context will then be passed into provider plugins
// to allow them to access configuration values and context variables.
func NewProviderContextFromParams(
	providerNamespace string,
	blueprintParams core.BlueprintParams,
) Context {
	return &providerCtxFromParams{
		providerNamespace: providerNamespace,
		blueprintParams:   blueprintParams,
	}
}

func (p *providerCtxFromParams) ProviderConfigVariable(name string) (*core.ScalarValue, bool) {
	providerConfig := p.blueprintParams.ProviderConfig(p.providerNamespace)
	if providerConfig == nil {
		return nil, false
	}

	configValue, ok := providerConfig[name]
	return configValue, ok
}

func (p *providerCtxFromParams) ContextVariable(name string) (*core.ScalarValue, bool) {
	contextVar := p.blueprintParams.ContextVariable(name)
	if contextVar == nil {
		return nil, false
	}
	return contextVar, true
}

type linkCtxFromParams struct {
	blueprintParams core.BlueprintParams
}

// NewLinkContextFromParams creates a new link context
// from a set of blueprint parameters for the current environment.
// The link context will then be passed into provider link plugins
// to allow them to access configuration values and context variables.
func NewLinkContextFromParams(
	blueprintParams core.BlueprintParams,
) LinkContext {
	return &linkCtxFromParams{
		blueprintParams: blueprintParams,
	}
}

func (p *linkCtxFromParams) ProviderConfigVariable(providerNamespace string, varName string) (*core.ScalarValue, bool) {
	providerConfig := p.blueprintParams.ProviderConfig(providerNamespace)
	if providerConfig == nil {
		return nil, false
	}

	configValue, ok := providerConfig[varName]
	return configValue, ok
}

func (p *linkCtxFromParams) ContextVariable(name string) (*core.ScalarValue, bool) {
	contextVar := p.blueprintParams.ContextVariable(name)
	if contextVar == nil {
		return nil, false
	}
	return contextVar, true
}

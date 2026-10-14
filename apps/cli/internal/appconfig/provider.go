package appconfig

import (
	"context"
	"fmt"
)

// NewConfigStore selects the right provider based on deploy target
// and creates a ConfigStore for the given store info.
func NewConfigStore(
	ctx context.Context,
	deployTarget string,
	info StoreInfo,
	opts ProviderOpts,
) (ConfigStore, error) {
	switch deployTarget {
	case "aws", "aws-serverless":
		return newAWSConfigStore(ctx, info, opts)
	default:
		return nil, fmt.Errorf(
			"config management not yet supported for deploy target %q", deployTarget,
		)
	}
}

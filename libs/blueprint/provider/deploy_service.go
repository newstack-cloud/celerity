package provider

import (
	"context"
)

// ResourceDeployService is an interface for a service that deploys resources.
// This is a subset of a resource registry that enables the deployment of resources
// in limited contexts that don't need the full functionality of a resource registry
// or a resource plugin implementation.
type ResourceDeployService interface {
	// Deploy deals with the deployment of a resource of a given type.
	Deploy(
		ctx context.Context,
		resourceType string,
		input *ResourceDeployInput,
	) (*ResourceDeployOutput, error)
	// Destroy deals with the destruction of a resource of a given type.
	Destroy(
		ctx context.Context,
		resourceType string,
		input *ResourceDestroyInput,
	) error
	// HasStabilised deals with checking if a resource has stabilised after being deployed.
	// This is important for resources that require a stable state before other resources can be deployed.
	// This is only used when creating or updating a resource, not when destroying a resource.
	HasStabilised(
		ctx context.Context,
		resourceType string,
		input *ResourceHasStabilisedInput,
	) (*ResourceHasStabilisedOutput, error)
}

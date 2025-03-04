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
	// Callers can specify whether or not to wait for the resource to stabilise
	// before returning.
	Deploy(
		ctx context.Context,
		resourceType string,
		input *ResourceDeployServiceInput,
	) (*ResourceDeployOutput, error)
	// Destroy deals with the destruction of a resource of a given type.
	Destroy(
		ctx context.Context,
		resourceType string,
		input *ResourceDestroyInput,
	) error
}

// ResourceDeployServiceInput is the input for the Deploy method of the ResourceDeployService
// that enhances the ResourceDeployInput with a flag to allow the caller
// to specify whether or not to wait for the resource to stabilise before returning.
type ResourceDeployServiceInput struct {
	// DeployInput is the input for the resource deployment that is passed into the `Deploy`
	// method of a `provider.Resource` implementation.
	DeployInput *ResourceDeployInput
	// WaitUntilStable specifies whether or not to
	// wait for the resource to stabilise before returning.
	WaitUntilStable bool
}

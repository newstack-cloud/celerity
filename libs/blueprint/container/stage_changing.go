package container

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

// ResourceChangeStager is an interface for a service that handles
// staging changes for a resource based on the current state of the
// resource, the resolved resource spec and the state definition
// provided by the resource plugin implementation.
type ResourceChangeStager interface {
	StageChanges(
		ctx context.Context,
		resourceInfo *provider.ResourceInfo,
	) (*provider.Changes, error)
}

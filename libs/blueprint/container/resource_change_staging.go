package container

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

// ResourceChangeStager is an interface for a service that handles
// staging changes for a resource based on the current state of the
// resource, the resolved resource spec and the spec definition
// provided by the resource plugin implementation.
type ResourceChangeStager interface {
	StageChanges(
		ctx context.Context,
		resourceInfo *provider.ResourceInfo,
		resourceImplementation provider.Resource,
	) (*provider.Changes, error)
}

type defaultResourceChangeStager struct{}

// NewResourceChangeStager returns a new instance of the default
// implementation of a resource change stager.
func NewDefaultResourceChangeStager() ResourceChangeStager {
	return &defaultResourceChangeStager{}
}

func (s *defaultResourceChangeStager) StageChanges(
	ctx context.Context,
	resourceInfo *provider.ResourceInfo,
	resourceImplementation provider.Resource,
) (*provider.Changes, error) {
	return nil, nil
}

package container

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/validation"
)

// ChildBlueprintDeployer provides an interface for a service that deploys a child
// blueprint as a part of the deployment process for a blueprint instance.
type ChildBlueprintDeployer interface {
	Deploy(
		ctx context.Context,
		instanceID string,
		instanceTreePath string,
		includeTreePath string,
		childNode *validation.ReferenceChainNode,
		changes *BlueprintChanges,
		deployCtx *DeployContext,
	)
}

// NewDefaultChildBlueprintDeployer creates a new instance of the default implementation
// of the service that deploys a child blueprint as a part of the deployment process
// for a blueprint instance.
func NewDefaultChildBlueprintDeployer() ChildBlueprintDeployer {
	return &defaultChildBlueprintDeployer{}
}

type defaultChildBlueprintDeployer struct{}

func (d *defaultChildBlueprintDeployer) Deploy(
	ctx context.Context,
	instanceID string,
	instanceTreePath string,
	includeTreePath string,
	childNode *validation.ReferenceChainNode,
	changes *BlueprintChanges,
	deployCtx *DeployContext,
) {
}

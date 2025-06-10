package testutils

import (
	"github.com/newstack-cloud/celerity/libs/blueprint/changes"
	"github.com/newstack-cloud/celerity/libs/blueprint/container"
)

type ChangeStagingEvent struct {
	ResourceChangesEvent  *container.ResourceChangesMessage
	ChildChangesEvent     *container.ChildChangesMessage
	LinkChangesEvent      *container.LinkChangesMessage
	FinalBlueprintChanges *changes.BlueprintChanges
	Error                 error
}

type DeployEventWrapper struct {
	DeployEvent *container.DeployEvent
	DeployError error
}

package typesv1

import (
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/params"
	"github.com/two-hundred/celerity/libs/blueprint-state/manage"
	"github.com/two-hundred/celerity/libs/blueprint/container"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/includes"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	commoncore "github.com/two-hundred/celerity/libs/common/core"
)

// Dependencies holds all the dependency services
// that are required by the controllers that provide HTTP handlers
// for v1 of the Deploy Engine API.
type Dependencies struct {
	EventStore        manage.Events
	ValidationStore   manage.Validation
	ChangesetStore    manage.Changesets
	Instances         state.InstancesContainer
	Exports           state.ExportsContainer
	IDGenerator       core.IDGenerator
	EventIDGenerator  core.IDGenerator
	ValidationLoader  container.Loader
	DeploymentLoader  container.Loader
	BlueprintResolver includes.ChildResolver
	ParamsProvider    params.Provider
	Clock             commoncore.Clock
	Logger            core.Logger
}

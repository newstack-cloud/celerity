package container

import (
	"time"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

// DeployContext holds information shared between components that handle different
// parts of the deployment process for a blueprint instance.
type DeployContext struct {
	StartTime  time.Time
	Rollback   bool
	Destroying bool
	State      DeploymentState
	Channels   *DeployChannels
	// A snapshot of the instance state taken before deployment.
	InstanceStateSnapshot *state.InstanceState
	ParamOverrides        core.BlueprintParams
	ResourceProviders     map[string]provider.Provider
	CurrentGroupIndex     int
	DeploymentGroups      [][]*DeploymentNode
}

func DeployContextWithChannels(
	deployCtx *DeployContext,
	channels *DeployChannels,
) *DeployContext {
	return &DeployContext{
		StartTime:             deployCtx.StartTime,
		State:                 deployCtx.State,
		Channels:              channels,
		Rollback:              deployCtx.Rollback,
		Destroying:            deployCtx.Destroying,
		InstanceStateSnapshot: deployCtx.InstanceStateSnapshot,
		ParamOverrides:        deployCtx.ParamOverrides,
		ResourceProviders:     deployCtx.ResourceProviders,
		CurrentGroupIndex:     deployCtx.CurrentGroupIndex,
		DeploymentGroups:      deployCtx.DeploymentGroups,
	}
}

func DeployContextWithGroup(
	deployCtx *DeployContext,
	groupIndex int,
) *DeployContext {
	return &DeployContext{
		StartTime:             deployCtx.StartTime,
		State:                 deployCtx.State,
		Channels:              deployCtx.Channels,
		Rollback:              deployCtx.Rollback,
		Destroying:            deployCtx.Destroying,
		InstanceStateSnapshot: deployCtx.InstanceStateSnapshot,
		ParamOverrides:        deployCtx.ParamOverrides,
		ResourceProviders:     deployCtx.ResourceProviders,
		CurrentGroupIndex:     groupIndex,
		DeploymentGroups:      deployCtx.DeploymentGroups,
	}
}

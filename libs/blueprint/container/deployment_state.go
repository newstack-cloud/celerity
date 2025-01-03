package container

import (
	"sync"
	"time"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

// DeploymentState provides functionality for tracking and setting the state of a deployment.
// In most cases, this is to be treated as ephemeral state that lasts for the duration
// of a deployment operation.
// This is not to be confused with the state of blueprint instances, which is persisted
// with implementations of the state.Container interface.
type DeploymentState interface {
	// SetDestroyedElement marks an element in the current
	// deployment as destroyed.
	SetDestroyedElement(element state.Element)
	// SetUpdatedEleemnt marks an element in the current
	// deployment as updated.
	SetUpdatedElement(element state.Element)
	// SetCreatedElement marks an element in the current
	// deployment as created.
	SetCreatedElement(element state.Element)
	// SetLinkDurationInfo stores the duration information for multiple stages
	// of the deployment of a link.
	SetLinkDurationInfo(linkName string, durations *state.LinkCompletionDurations)
	// GetLinkDurationInfo returns the duration information for multiple stages
	// of the deployment of a link.
	GetLinkDurationInfo(linkName string) *state.LinkCompletionDurations
	// SetPrepareDuration sets the duration of the preparation phase for the deployment
	// of a blueprint instance.
	SetPrepareDuration(prepareDuration time.Duration)
	// GetPrepareDuration returns the duration of the preparation phase for the deployment
	// of a blueprint instance.
	GetPrepareDuration() *time.Duration
	// SetResourceSpecState sets the spec state for a resource that has been created
	// or updated.
	SetResourceSpecState(resourceName string, specState *core.MappingNode)
}

// NewDefaultDeploymentState creates a new instance of the default
// implementation for tracking and setting the state of a deployment.
// The default implementation is a thread-safe, ephemeral store for deployment state.
func NewDefaultDeploymentState() DeploymentState {
	return &defaultDeploymentState{
		pendingLinks:               make(map[string]*LinkPendingCompletion),
		resourceNamePendingLinkMap: make(map[string][]string),
		destroyed:                  make(map[string]state.Element),
		created:                    make(map[string]state.Element),
		resourceSpecStates:         make(map[string]*core.MappingNode),
		updated:                    make(map[string]state.Element),
		linkDurationInfo:           make(map[string]*state.LinkCompletionDurations),
	}
}

// Keeps track of state regarding when links are ready to be processed
// along with elements that have been successfully processed.
// All instance state including statuses of resources, links and child blueprints
// are stored in the state container.
// This is a temporary representation of the state of the deployment
// that is not persisted.
type defaultDeploymentState struct {
	// A mapping of a logical link name to the pending link completion state for links
	// that need to be deployed or updated.
	// A link ID in this context is made up of the resource names of the two resources
	// that are linked together.
	// For example, if resource A is linked to resource B, the link name would be "A::B".
	// This is used to keep track of when links are ready to be deployed or updated.
	// This does not hold the state of the link, only information about when the link is ready
	// to be deployed or updated.
	// Link removals are not tracked here as they do not depend on resource state changes,
	// removal of links happens before resources in the link relationship are processed.
	pendingLinks map[string]*LinkPendingCompletion
	// A mapping of resource names to pending links that include the resource.
	resourceNamePendingLinkMap map[string][]string
	// Elements that have been successfully destroyed.
	// This is a mapping of namespaced logical names (e.g. resources.resourceA) to an element
	// representing identifiers and the kind of the element.
	destroyed map[string]state.Element
	// Elements that have been successfully created/deployed.
	// This is a mapping of namespaced logical names (e.g. resources.resourceA) to an element
	// representing identifiers and the kind of the element.
	created map[string]state.Element
	// Holds the returned spec state for resources that have been deployed.
	resourceSpecStates map[string]*core.MappingNode
	// Elements that have been successfully updated.
	// This is a mapping of namespaced logical names (e.g. resources.resourceA) to an element
	// representing identifiers and the kind of the element.
	updated map[string]state.Element
	// The duration of the preparation phase for the deployment of a blueprint instance.
	prepareDuration *time.Duration
	// A mapping of logical link name to the current duration information for the progress
	// of the link deployment.
	linkDurationInfo map[string]*state.LinkCompletionDurations
	// Mutex is required as resources can be deployed concurrently.
	mu sync.Mutex
}

func (d *defaultDeploymentState) SetDestroyedElement(element state.Element) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.destroyed[getNamespacedLogicalName(element)] = element
}

func (d *defaultDeploymentState) SetUpdatedElement(element state.Element) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.updated[getNamespacedLogicalName(element)] = element
}

func (d *defaultDeploymentState) SetCreatedElement(element state.Element) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.created[getNamespacedLogicalName(element)] = element
}

func (d *defaultDeploymentState) SetLinkDurationInfo(linkName string, durations *state.LinkCompletionDurations) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.linkDurationInfo[linkName] = copyLinkCompletionDurations(durations)
}

func (d *defaultDeploymentState) GetLinkDurationInfo(linkName string) *state.LinkCompletionDurations {
	d.mu.Lock()
	defer d.mu.Unlock()

	durationInfo, hasDurationInfo := d.linkDurationInfo[linkName]
	if !hasDurationInfo {
		return &state.LinkCompletionDurations{}
	}

	// Make a copy of durations so any modifications made to the returned value
	// does not affect the value in the deployment state.
	return copyLinkCompletionDurations(durationInfo)
}

func (d *defaultDeploymentState) SetPrepareDuration(prepareDuration time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.prepareDuration = &prepareDuration
}

func (d *defaultDeploymentState) GetPrepareDuration() *time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.prepareDuration
}

func (d *defaultDeploymentState) SetResourceSpecState(resourceName string, specState *core.MappingNode) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.resourceSpecStates[resourceName] = specState
}

func copyLinkCompletionDurations(durations *state.LinkCompletionDurations) *state.LinkCompletionDurations {
	if durations == nil {
		return &state.LinkCompletionDurations{}
	}

	return &state.LinkCompletionDurations{
		ResourceAUpdate:       copyLinkComponentCompletionDurations(durations.ResourceAUpdate),
		ResourceBUpdate:       copyLinkComponentCompletionDurations(durations.ResourceBUpdate),
		IntermediaryResources: copyLinkComponentCompletionDurations(durations.IntermediaryResources),
	}
}

func copyLinkComponentCompletionDurations(durations *state.LinkComponentCompletionDurations) *state.LinkComponentCompletionDurations {
	if durations == nil {
		return nil
	}

	return &state.LinkComponentCompletionDurations{
		TotalDuration:    durations.TotalDuration,
		AttemptDurations: append([]float64{}, durations.AttemptDurations...),
	}
}

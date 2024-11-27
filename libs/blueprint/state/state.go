package state

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
)

// Container provides an interface for services
// that persist blueprint instance state.
// Various methods are provided to deal with extracting and saving information
// for a blueprint instance.
// Instead of operating at the instance level and manipulating the entire state,
// methods are provided to deal with sub-components of the instance state
// such as resources, links, metadata and exports.
// Depending on the implementation, it can be more efficient to deal with
// sub-components of the instance state separately.
// Fr example, `GetInstance` may be a view of the instance state
// that could be an expensive operation to perform involving multiple
// queries to a database or an expensive join operation.
// The state persistence method is entirely up to the application
// making use of this library.
type Container interface {
	// GetInstance deals with retrieving the state for a given blueprint
	// instance ID.
	GetInstance(ctx context.Context, instanceID string) (InstanceState, error)
	// SaveInstance deals with persisting a blueprint instance.
	SaveInstance(ctx context.Context, instanceState InstanceState) error
	// RemoveInstance deals with removing the state for a given blueprint instance.
	// This is not for destroying the actual deployed resources, just removing the state.
	RemoveInstance(ctx context.Context, instanceID string) (InstanceState, error)
	// GetResource deals with retrieving the state for a given resource
	// in the provided blueprint instance.
	GetResource(ctx context.Context, instanceID string, resourceID string) (ResourceState, error)
	// GetResourceByName deals with retrieving the state for a given resource
	// in the provided blueprint instance by its logical name.
	GetResourceByName(ctx context.Context, instanceID string, resourceName string) (ResourceState, error)
	// SaveResource deals with persisting a resource in a blueprint instance.
	SaveResource(
		ctx context.Context,
		instanceID string,
		resourceState ResourceState,
	) error
	// RemoveResource deals with removing the state of a resource from
	// a given blueprint instance.
	RemoveResource(ctx context.Context, instanceID string, resourceID string) (ResourceState, error)
	// GetResourceDrift deals with retrieving the current drift state for a given resource
	// in the provided blueprint instance.
	GetResourceDrift(ctx context.Context, instanceID string, resourceID string) (ResourceState, error)
	// SaveResourceDrift deals with persisting the drift state for a given resource
	// in the provided blueprint instance.
	SaveResourceDrift(ctx context.Context, instanceID string, driftState ResourceState) error
	// RemoveResourceDrift deals with removing the drift state for a given resource
	// in the provided blueprint instance.
	RemoveResourceDrift(ctx context.Context, instanceID string, resourceID string) (ResourceState, error)
	// GetLink deals with retrieving the state for a given link
	// in the provided blueprint instance.
	GetLink(ctx context.Context, instanceID string, linkID string) (LinkState, error)
	// SaveLink deals with persisting a link in a blueprint instance.
	SaveLink(ctx context.Context, instanceID string, linkState LinkState) error
	// RemoveLink deals with removing the state of a link from
	// a given blueprint instance.
	RemoveLink(ctx context.Context, instanceID string, linkID string) (LinkState, error)
	// GetMetadata deals with retrieving metadata for a given blueprint instance.
	GetMetadata(ctx context.Context, instanceID string) (map[string]*core.MappingNode, error)
	// SaveMetadata deals with persisting metadata for a given blueprint instance.
	SaveMetadata(ctx context.Context, instanceID string, metadata map[string]*core.MappingNode) error
	// RemoveMetadata deals with removing metadata from a given blueprint instance.
	RemoveMetadata(ctx context.Context, instanceID string) (map[string]*core.MappingNode, error)
	// GetExports deals with retrieving exported fields for a given blueprint instance.
	GetExports(ctx context.Context, instanceID string) (map[string]*core.MappingNode, error)
	// GetExport deals with retrieving an exported field for a given blueprint instance.
	GetExport(ctx context.Context, instanceID string, exportName string) (*core.MappingNode, error)
	// SaveExports deals with persisting exported fields for a given blueprint instance.
	SaveExports(ctx context.Context, instanceID string, exports map[string]*core.MappingNode) error
	// SaveExport deals with persisting an exported field for a given blueprint instance.
	SaveExport(ctx context.Context, instanceID string, exportName string, export *core.MappingNode) error
	// RemoveExports deals with removing all exported fields for a given blueprint instance.
	RemoveExports(ctx context.Context, instanceID string) (map[string]*core.MappingNode, error)
	// RemoveExport deals with removing an exported field for a given blueprint instance.
	RemoveExport(ctx context.Context, instanceID string, exportName string) (*core.MappingNode, error)
	// GetChild deals with retrieving the state for a given child blueprint
	// in the provided blueprint instance.
	GetChild(ctx context.Context, instanceID string, childName string) (InstanceState, error)
	// SaveChild deals with persisting a blueprint instance and assigning
	// it as a child of the provided blueprint instance.
	SaveChild(ctx context.Context, instanceID string, childName string, childState InstanceState) error
	// RemoveChild deals with removing the state of a child blueprint from
	// a given blueprint instance.
	RemoveChild(ctx context.Context, instanceID string, childName string) (InstanceState, error)
}

// ResourceState provides the current state of a resource
// in a blueprint instance.
// This includes the status, the Raw data from the upstream resouce provider
// along with reasons for failure when a resource is in a failure state.
type ResourceState struct {
	// A globally unique identifier for the resource.
	ResourceID string `json:"resourceId"`
	// The logical name of the resource in the blueprint.
	ResourceName string `json:"resourceName"`
	// The name of the resource template in the source blueprint
	// that the resource is derived from.
	// This will be empty if the resource is not derived from a resource template.
	ResourceTemplateName string `json:"resourceTemplateName,omitempty"`
	// The index of the resource template in the source blueprint
	// that the resource is derived from.
	// This will be empty if the resource is not derived from a resource template.
	ResourceTemplateIndex *int                       `json:"resourceTemplateIndex,omitempty"`
	Status                core.ResourceStatus        `json:"status"`
	PreciseStatus         core.PreciseResourceStatus `json:"preciseStatus"`
	// LastDeployedTimestamp holds the unix timestamp when the resource was last deployed.
	LastDeployedTimestamp int `json:"lastDeployedTimestamp"`
	// LastDeployAttempTimestamp holds the unix timestamp when an attempt was last made to deploy the resource.
	LastDeployAttemptTimestamp int `json:"lastDeployAttemptTimestamp"`
	// ResourceSpecData holds the resolved resource spec
	// for the currently deployed version of the resource along with computed
	// fields derived from the deployed resource in the provider.
	ResourceSpecData *core.MappingNode `json:"resourceSpecData"`
	// Metadata holds metadata for the resource that is derived from a source blueprint
	// that includes additional information that allows for extensions built on top of the
	// blueprint framework along with the storage of labels, annotations and a human-friendly
	// display name for the resource.
	Metadata *ResourceMetadataState `json:"metadata,omitempty"`
	// Holds the latest reasons for failures in deploying a resource,
	// this only ever holds the results of the latest deployment attempt.
	FailureReasons []string `json:"failureReasons"`
	// Drifted indicates whether or not the resource state has drifted
	// due to changes in the upstream provider.
	Drifted bool `json:"drifted,omitempty"`
	// LastDriftDetectedTimestamp holds the unix timestamp when drift was last detected.
	LastDriftDetectedTimestamp *int `json:"lastDriftDetectedTimestamp,omitempty"`
}

// ResourceMetadataState holds metadata for a resource
// that is derived from a source blueprint.
type ResourceMetadataState struct {
	DisplayName string            `json:"displayName,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Custom      *core.MappingNode `json:"custom,omitempty"`
}

// InstanceState stores the state of a blueprint instance
// including resources, metadata, exported fields and child blueprints.
type InstanceState struct {
	InstanceID string
	Status     core.InstanceStatus
	// LastDeployedTimestamp holds the unix timestamp when the blueprint instance was last deployed.
	LastDeployedTimestamp int `json:"lastDeployedTimestamp"`
	// LastDeployAttempTimestamp holds the unix timestamp when an attempt
	// was last made to deploy the blueprint instance.
	LastDeployAttemptTimestamp int `json:"lastDeployAttemptTimestamp"`
	// A mapping of logical resource definition name
	// to the ordered list of resource IDs
	// that are created from the resource definition.
	ResourceIDs map[string]string
	Resources   map[string]*ResourceState
	Links       map[string]*LinkState
	// Metadata is used internally to store additional non-structured information
	// that is relevant to the blueprint framework but can also be used to store
	// additional information that is relevant to the application/tool
	// making use of this library.
	Metadata        map[string]*core.MappingNode
	Exports         map[string]*core.MappingNode
	ChildBlueprints map[string]*InstanceState
	// Drifted indicates whether or not the blueprint instance has drifted
	// due to changes to resources in the upstream provider.
	Drifted bool
	// LastDriftDetectedTimestamp holds the unix timestamp when drift in any of the resources
	// in the blueprint instance was last detected.
	LastDriftDetectedTimestamp *int `json:"lastDriftDetectedTimestamp,omitempty"`
}

// LinkState provides a way to store some state for links between
// resources.
// This is useful for holding state about intermediary resources
// managed by a provider's implementation of a link.
type LinkState struct {
	// A globally unique identifier for the link.
	LinkID        string
	Status        core.LinkStatus
	PreciseStatus core.PreciseLinkStatus
	// LastDeployedTimestamp holds the unix timestamp when the link was last deployed.
	LastDeployedTimestamp int `json:"lastDeployedTimestamp"`
	// LastDeployAttempTimestamp holds the unix timestamp when an attempt was last made to deploy the link.
	LastDeployAttemptTimestamp int `json:"lastDeployAttemptTimestamp"`
	// IntermediaryResourceStates holds the state of intermediary resources
	// that are created by the provider's implementation of a link.
	IntermediaryResourceStates []*ResourceState
	// ResourceData is the mapping that holds the structure of
	// the "raw" link data to hold information about a link that is not
	// stored directly in the resources that are linked and is not
	// stored in intermediary resources.
	LinkData map[string]*core.MappingNode
}

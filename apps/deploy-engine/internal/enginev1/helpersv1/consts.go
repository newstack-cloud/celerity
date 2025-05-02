package helpersv1

const (
	// ChannelTypeValidation is the channel type identifier
	// for validation events.
	ChannelTypeValidation = "validation"
	// ChannelTypeChangeset is the channel type identifier
	// for change staging (change set) events.
	ChannelTypeChangeset = "changeset"
	// ChannelTypeDeployment is the channel type identifier
	// for deployment events.
	ChannelTypeDeployment = "deployment"
	// LastEventIDHeader is the name of the HTTP header
	// that can contain the last event ID for SSE.
	LastEventIDHeader = "Last-Event-ID"
)

// Default values for shared request payload fields.
const (
	// DefaultFileSourceScheme is the default file source scheme
	// used for requests that need to fetch a blueprint document.
	DefaultFileSourceScheme = "file"
	// DefaultBlueprintFile is the default name of the blueprint file
	// used for requests that need to fetch a blueprint document.
	DefaultBlueprintFile = "project.blueprint.yml"
)

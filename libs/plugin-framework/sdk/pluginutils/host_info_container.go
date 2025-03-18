package pluginutils

// HostInfoContainer is an interface that represents a container for host information.
type HostInfoContainer interface {
	// GetID returns the ID of the host system that the plugin is running for.
	GetID() string
	// SetID sets the ID of the host system that the plugin is running for.
	SetID(id string)
}

// hostInfoContainerImpl is an implementation of HostInfoContainer.
// This should only be written to once during initialisation and
// should be read-only after that.
type hostInfoContainerImpl struct {
	id string
}

// NewHostInfoContainer creates a new container for host information
// that is used by a plugin built with the plugin SDK to carry out tasks
// like check that the host making requests is the same as the host the plugin
// was registered with on initialisation.
func NewHostInfoContainer() HostInfoContainer {
	return &hostInfoContainerImpl{}
}

// GetID returns the ID of the host system that the plugin is running for.
func (c *hostInfoContainerImpl) GetID() string {
	return c.id
}

// SetID sets the ID of the host system that the plugin is running for.
func (c *hostInfoContainerImpl) SetID(id string) {
	c.id = id
}

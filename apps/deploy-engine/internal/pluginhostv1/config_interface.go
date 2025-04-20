package pluginhostv1

// Config provides an interface for a plugin host
// configuration provider.
type Config interface {
	// GetPluginPath returns the path to
	// one or more plugin root directories.
	GetPluginPath() string
	// GetLaunchWaitTimeoutMS returns the timeout in milliseconds
	// for waiting for a plugin to register with the host.
	GetLaunchWaitTimeoutMS() int
	// GetTotalLaunchWaitTimeoutMS returns the timeout in milliseconds
	// for waiting for all plugins to register with the host.
	GetTotalLaunchWaitTimeoutMS() int
	// GetResourceStabilisationPollingTimeoutMS
	// returns the timeout in milliseconds
	// to wait for a resource to stabilise when calls are made
	// into the resource registry through the plugin service.
	// This will be used when a link plugin makes calls to deploy resources
	// via the ResourceDeployService.
	GetResourceStabilisationPollingTimeoutMS() int
	// GetResourceStabilisationPollingIntervalMS
	// returns the interval in milliseconds
	// to poll for a resource to stabilise when calls are made
	// into the resource registry through the plugin service.
	// This will be used when a link plugin makes calls to deploy resources
	// via the ResourceDeployService.
	GetResourceStabilisationPollingIntervalMS() int
	// GetPluginToPluginCallTimeoutMS returns the timeout in milliseconds
	// for waiting for a plugin to respond to a call initiated by another
	// or the same plugin through the plugin service.
	GetPluginToPluginCallTimeoutMS() int
}

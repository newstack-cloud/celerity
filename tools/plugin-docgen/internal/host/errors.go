package host

import "errors"

var (
	// ErrPluginNotFound is returned when a plugin could not be found
	// after launching plugins for the host service.
	ErrPluginNotFound = errors.New("plugin not found")
)

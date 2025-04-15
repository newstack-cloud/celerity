package docgen

import "errors"

var (
	// ErrInvalidPluginType is returned when the resolved plugin is not
	// of the allowed types.
	ErrInvalidPluginType = errors.New(
		"invalid plugin type for plugin instance, plugin must implement one of the " +
			"`provider.Provider` or `transform.SpecTransformer` interfaces",
	)

	// ErrPluginMetadataNotFound is returned when the plugin metadata
	// could not be found in the plugin manager.
	ErrPluginMetadataNotFound = errors.New(
		"plugin metadata not found, this is required to produce documentation",
	)
)

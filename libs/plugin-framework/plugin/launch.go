package plugin

import (
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
)

// PluginMaps is a set of adaptors that can be used as maps of providers
// and transformers to be used to create a blueprint loader.
type PluginMaps struct {
	Providers    map[string]provider.Provider
	Transformers map[string]transform.SpecTransformer
}

// LaunchPlugins discovers, executes plugin binaries
// and waits for N plugins to have registered with the host service.
// This returns a set of adaptors that can be used as maps of providers
// and transformers to be used to create a blueprint loader.
func LaunchPlugins() (*PluginMaps, error) {
	// 1. discover plugin binaries
	// 2. execute plugin binaries
	// 3. wait for N plugins to have registered with the host service
	// 4. create adaptors for the host service to interact with the plugins
	// 5. return adaptors for the deploy engine to set up the blueprint loader
	return &PluginMaps{}, nil
}

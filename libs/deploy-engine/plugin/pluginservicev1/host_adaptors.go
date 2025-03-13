// This file provides adaptors for the host service to interact
// with the plugin service manager using the blueprint framework
// interfaces for providers and transformer plugins.
package pluginservicev1

import (
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

// GetProviderPluginAdaptors returns a map of provider adaptors
// that can be used by the host service to interact with
// registered provider plugins.
func GetProviderPluginAdaptors(manager Manager) map[string]provider.Provider {
	providerPlugins := manager.GetPlugins(PluginType_PLUGIN_TYPE_PROVIDER)
	adaptors := make(map[string]provider.Provider)

	for _, plugin := range providerPlugins {
		namespace := extractProviderNamespaceFromID(plugin.Info.ID)
		// The factory used by the manager is expected to wrap the plugin clients
		// with adaptors that produce an implementation of the provider.Provider interface.
		providerPlugin, isProvider := plugin.Client.(provider.Provider)
		if isProvider {
			adaptors[namespace] = providerPlugin
		}
	}

	return adaptors
}

func extractProviderNamespaceFromID(id string) string {
	// The ID is in the format {hostname/}?{namespace}/{provider}.
	// We need to extract the provider name used as the namespace
	// for entities managed by the provider plugin.
	// For example, the namespace for AWS resources is "aws"
	// in a plugin with the ID "celerity/aws"
	// used in the resource type "aws/lambda/function".
	lastSepIndex := strings.LastIndex(id, "/")
	if lastSepIndex == -1 {
		return ""
	}

	return id[lastSepIndex+1:]
}

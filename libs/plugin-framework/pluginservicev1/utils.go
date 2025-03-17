package pluginservicev1

// PluginTypeFromString converts a string to a PluginType.
// If the string is not recognized, PLUGIN_TYPE_NONE is returned.
func PluginTypeFromString(typeString string) PluginType {
	switch typeString {
	case "provider":
		return PluginType_PLUGIN_TYPE_PROVIDER
	case "transformer":
		return PluginType_PLUGIN_TYPE_TRANSFORMER
	default:
		return PluginType_PLUGIN_TYPE_NONE
	}
}

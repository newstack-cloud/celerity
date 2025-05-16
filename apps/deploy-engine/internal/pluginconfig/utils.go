package pluginconfig

// ToConfigDefinitionProviders is a helper function that converts a map of
// plugins (either transformers or providers) to a map of the simplified
// PluginConfigDefinitionProvider interface.
// Maps and slices of an interface aren't interchangeable between super
// type and subtypes.
func ToConfigDefinitionProviders[PluginSuperType DefinitionProvider](
	providers map[string]PluginSuperType,
) map[string]DefinitionProvider {
	converted := make(map[string]DefinitionProvider, len(providers))
	for k, v := range providers {
		converted[k] = v
	}
	return converted
}

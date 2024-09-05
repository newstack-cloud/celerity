package provider

import "strings"

// ExtractProviderFromItemType extracts the provider namespace from a resource type
// or data source type.
func ExtractProviderFromItemType(itemType string) string {
	parts := strings.Split(itemType, "/")
	if len(parts) == 0 {
		return ""
	}

	return parts[0]
}

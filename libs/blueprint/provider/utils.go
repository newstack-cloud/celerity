package provider

import "strings"

// ExtractProviderFromResourceType extracts the provider namespace from a resource type.
func ExtractProviderFromResourceType(resourceType string) string {
	parts := strings.Split(resourceType, "/")
	if len(parts) == 0 {
		return ""
	}

	return parts[0]
}

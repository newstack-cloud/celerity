package utils

// GetKeys returns the keys of a map as a slice of strings.
func GetKeys[Item any](m map[string]Item) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

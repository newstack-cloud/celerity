package corefunctions

// The function environment expects lists to be passed around as []interface{}
// where concrete types are asserted by functions that consume them.
func intoInterfaceSlice[Type any](slice []Type) []interface{} {
	result := make([]interface{}, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}

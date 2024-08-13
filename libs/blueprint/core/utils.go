package core

// IsInScalarList checks if a given scalar value is in a list of scalar values.
func IsInScalarList(value *ScalarValue, list []*ScalarValue) bool {
	found := false
	i := 0
	for !found && i < len(list) {
		found = list[i].Equal(value)
		i += 1
	}
	return found
}

package core

// MapToSlice converts a map to a slice of values.
// Order is not guaranteed so do not use this where
// the order of items in the slice matters.
func MapToSlice[Value any](inputMap map[string]Value) []Value {
	var items []Value
	for _, value := range inputMap {
		items = append(items, value)
	}
	return items
}

// SliceToMapKeys converts a slice of strings to a map where each item
// in the slice is a key in the map for an empty slice of values.
// This only works where the values in a map are slices!
func SliceToMapKeys[Value any](inputKeys []string) map[string][]Value {
	mapping := map[string][]Value{}
	for _, key := range inputKeys {
		mapping[key] = []Value{}
	}
	return mapping
}

// Comparable provides a custom interface for more complex equality
// checks between two values of the same type.
type Comparable[Value any] interface {
	Equal(value Value) bool
}

// SliceContains determines whether the provided slice
// contains the given searchFor value.
func SliceContains[Value Comparable[Value]](input []Value, searchFor Value) bool {
	found := false
	i := 0
	for !found && i < len(input) {
		found = input[i].Equal(searchFor)
		i += 1
	}
	return found
}

// SliceContainsComparable determines whether the provided slice
// contains the given searchFor value where the value fulfils the built-in
// comparable interface, complex structs will not fulfil this interface and therefore
// you should implement the Equal() interface and use SliceContains.
func SliceContainsComparable[Value comparable](input []Value, searchFor Value) bool {
	found := false
	i := 0
	for !found && i < len(input) {
		found = input[i] == searchFor
		i += 1
	}
	return found
}

// Map deals with mapping a slice of Inputs to a slice of Outputs with the given
// mapping function.
func Map[Input any, Output any](inputs []Input, mapFunc func(input Input, index int) Output) []Output {
	outputs := []Output{}
	for index, item := range inputs {
		outputs = append(outputs, mapFunc(item, index))
	}
	return outputs
}

// Filter deals with filtering a slice of items down to all those items that
// meet the predicate of the filter function.
func Filter[CollectionItem any](items []CollectionItem, filterFunc func(item CollectionItem, index int) bool) []CollectionItem {
	filtered := []CollectionItem{}
	for index, item := range items {
		keep := filterFunc(item, index)
		if keep {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// Find deals with finding the first occurence of an item in a given slice of items.
// For complex data types CollectionItem should be a pointer.
func Find[CollectionItem any](items []CollectionItem, findFunc func(item CollectionItem, index int) bool) CollectionItem {
	found := false
	i := 0
	var foundItem CollectionItem
	for !found && i < len(items) {
		found = findFunc(items[i], i)
		if found {
			foundItem = items[i]
		}
		i += 1
	}

	return foundItem
}

// FindIndex deals with finding the index of the first occurence of an item in a given slice of items.
// Returns -1 when item can not be found.
func FindIndex[CollectionItem any](items []CollectionItem, findFunc func(item CollectionItem, index int) bool) int {
	foundIndex := -1
	i := 0
	for foundIndex == -1 && i < len(items) {
		matches := findFunc(items[i], i)
		if matches {
			foundIndex = i
		}
		i += 1
	}

	return foundIndex
}

// RemoveDuplicates deals with removing duplicating values in the given slice.
// This only supports value types that fulfil the built-in comparable interface.
func RemoveDuplicates[Value comparable](input []Value) []Value {
	withoutDuplicates := []Value{}
	for _, value := range input {
		if !SliceContainsComparable(withoutDuplicates, value) {
			withoutDuplicates = append(withoutDuplicates, value)
		}
	}
	return withoutDuplicates
}

// ShallowCopyMap deals with making a copy of a map, shallow copying
// the values into the new map.
func ShallowCopyMap[Key comparable, Value any](inputMap map[Key]Value) map[Key]Value {
	copyMap := make(map[Key]Value)
	for key, value := range inputMap {
		copyMap[key] = value
	}
	return copyMap
}

// Reverse deals with producing a copy of a given slice
// with items in the reverse order.
// This does not mutate the input slice!
func Reverse[Value any](items []Value) []Value {
	newItems := make([]Value, len(items))
	for i := 0; i < len(items); i += 1 {
		j := len(items) - 1 - i
		newItems[j] = items[i]
	}
	return newItems
}

// SlicesEqual determines whether to slices of the same comparable
// type are equal in value.
func SlicesEqual[Value comparable](a []Value, b []Value) bool {
	if len(a) != len(b) {
		return false
	}

	matches := true
	i := 0
	for matches && i < len(a) {
		matches = a[i] == b[i]
		i += 1
	}

	return matches
}

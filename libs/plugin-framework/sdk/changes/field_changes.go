package changes

import (
	"github.com/newstack-cloud/celerity/libs/blueprint/core"
	"github.com/newstack-cloud/celerity/libs/blueprint/provider"
)

// TODO: create a more general field change look up utility that can be used
// to get new values, previous values, and collect a subset of field changes
// based on a shared prefix. (e.g. name of a map or list field)
// You should be able to extract slices and maps of new values and previous values
// based on a prefix.

// FieldChangesToNewValueMap converts a slice of FieldChange structs to a map
// where the keys are the field paths and the values are pointers to the
// corresponding new value for the field. This is useful for creating lookups
// for potentially large sets of field changes, allowing for quick access
// to new values by field paths.
//
// Multiple slices of field changes can be provided, as a convenience
// when merging new and modified field changes into a single map.
func FieldChangesToNewValueMap(
	changes ...[]provider.FieldChange,
) map[string]*core.MappingNode {
	allChanges := []provider.FieldChange{}
	for _, changeSlice := range changes {
		allChanges = append(allChanges, changeSlice...)
	}

	fieldChangeMap := make(map[string]*core.MappingNode, len(allChanges))
	for _, change := range allChanges {
		if change.NewValue != nil {
			fieldChangeMap[change.FieldPath] = change.NewValue
		}
	}

	return fieldChangeMap
}

// FieldChangesToPrevValueMap converts a slice of FieldChange structs to a map
// where the keys are the field paths and the values are pointers to the
// corresponding previous value for the field. This is useful for creating lookups
// for potentially large sets of field changes, allowing for quick access
// to previous values by field paths.
//
// Multiple slices of field changes can be provided, as a convenience
// when merging new and modified field changes into a single map.
func FieldChangesToPrevValueMap(
	changes ...[]provider.FieldChange,
) map[string]*core.MappingNode {
	allChanges := []provider.FieldChange{}
	for _, changeSlice := range changes {
		allChanges = append(allChanges, changeSlice...)
	}

	fieldChangeMap := make(map[string]*core.MappingNode, len(allChanges))
	for _, change := range allChanges {
		if change.PrevValue != nil {
			fieldChangeMap[change.FieldPath] = change.PrevValue
		}
	}

	return fieldChangeMap
}

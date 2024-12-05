package internal

import (
	"slices"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

// NormaliseResourceChanges normalises the order of fields in a provider.Changes struct
// so that it can be compared deterministically in tests.
func NormaliseResourceChanges(changes *provider.Changes, excludeResourceInfo bool) *provider.Changes {
	appliedResourceInfo := changes.AppliedResourceInfo
	if excludeResourceInfo {
		appliedResourceInfo = provider.ResourceInfo{}
	}

	return &provider.Changes{
		AppliedResourceInfo:       appliedResourceInfo,
		MustRecreate:              changes.MustRecreate,
		ModifiedFields:            orderFieldChanges(changes.ModifiedFields),
		NewFields:                 orderFieldChanges(changes.NewFields),
		RemovedFields:             orderStringSlice(changes.RemovedFields),
		UnchangedFields:           orderStringSlice(changes.UnchangedFields),
		FieldChangesKnownOnDeploy: orderStringSlice(changes.FieldChangesKnownOnDeploy),
		NewOutboundLinks:          changes.NewOutboundLinks,
		OutboundLinkChanges:       changes.OutboundLinkChanges,
		RemovedOutboundLinks:      changes.RemovedOutboundLinks,
	}
}

func orderStringSlice(fields []string) []string {
	orderedFields := make([]string, len(fields))
	copy(orderedFields, fields)
	slices.Sort(orderedFields)
	return orderedFields
}

func orderFieldChanges(fieldChanges []provider.FieldChange) []provider.FieldChange {
	orderedFieldChanges := make([]provider.FieldChange, len(fieldChanges))
	copy(orderedFieldChanges, fieldChanges)
	slices.SortFunc(orderedFieldChanges, func(a, b provider.FieldChange) int {
		if a.FieldPath < b.FieldPath {
			return -1
		}

		if a.FieldPath > b.FieldPath {
			return 1
		}

		return 0
	})
	return orderedFieldChanges
}

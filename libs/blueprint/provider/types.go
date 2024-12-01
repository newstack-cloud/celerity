package provider

import "github.com/two-hundred/celerity/libs/blueprint/core"

// FieldChange represents a change in a field value
// of a resource or link that is used in change staging.
type FieldChange struct {
	FieldName string            `json:"fieldName"`
	PrevValue *core.MappingNode `json:"prevValue"`
	NewValue  *core.MappingNode `json:"newValue"`
	// MustRecreate is a flag that indicates whether the resource or link
	// containing the field must be recreated in order to apply the change.
	MustRecreate bool `json:"mustRecreate"`
}

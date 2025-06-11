package pluginutils

import (
	"github.com/newstack-cloud/celerity/libs/blueprint/core"
)

// SpecValueSetter is a helper struct that can be used to set a value in a
// from a resource spec to a target API-specific struct used to update
// or create the resource in the upstream provider.
type SpecValueSetter[Target any] struct {
	PathInSpec   string
	SetValueFunc func(
		value *core.MappingNode,
		target Target,
	)
	didSet bool
}

func (u *SpecValueSetter[Target]) Set(
	value *core.MappingNode,
	target Target,
) {
	value, hasValue := GetValueByPath(u.PathInSpec, value)
	if !hasValue {
		return
	}

	u.SetValueFunc(value, target)
	u.didSet = true
}

func (u *SpecValueSetter[Target]) DidSet() bool {
	return u.didSet
}

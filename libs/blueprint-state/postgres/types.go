package postgres

import "github.com/two-hundred/celerity/libs/blueprint/state"

type descendantBlueprintInfo struct {
	parentInstanceID  string
	childInstanceName string
	childInstanceID   string
	instance          state.InstanceState
}

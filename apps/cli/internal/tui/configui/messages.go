package configui

import (
	"github.com/newstack-cloud/celerity/apps/cli/internal/appconfig"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devconfig"
)

// EnvsLoadedMsg is sent when environments have been discovered.
type EnvsLoadedMsg struct {
	Envs []devconfig.EnvironmentInfo
}

// EnvsLoadErrorMsg is sent when environment discovery fails.
type EnvsLoadErrorMsg struct {
	Err error
}

// EnvSelectedMsg is sent when the user selects an environment.
type EnvSelectedMsg struct {
	Env devconfig.EnvironmentInfo
}

// StoresLoadedMsg is sent when stores have been extracted from the blueprint.
type StoresLoadedMsg struct {
	Stores []appconfig.StoreInfo
}

// StoresLoadErrorMsg is sent when store extraction fails.
type StoresLoadErrorMsg struct {
	Err error
}

// StoreSelectedMsg is sent when the user selects a store.
type StoreSelectedMsg struct {
	Store appconfig.StoreInfo
}

// KeysLoadedMsg is sent when keys are loaded from the config store.
type KeysLoadedMsg struct {
	Entries []appconfig.ConfigEntry
}

// KeysLoadErrorMsg is sent when key loading fails.
type KeysLoadErrorMsg struct {
	Err error
}

// KeySetMsg is sent after a key has been set successfully.
type KeySetMsg struct {
	Key string
}

// KeyDeletedMsg is sent after a key has been deleted successfully.
type KeyDeletedMsg struct {
	Key string
}

// OperationErrorMsg is sent when a config operation fails.
type OperationErrorMsg struct {
	Err error
}

package appconfig

import "go.uber.org/zap"

// StoreKind identifies the cloud provider storage backend for a config store.
type StoreKind string

const (
	StoreKindSecretsManager StoreKind = "secrets-manager"
	StoreKindParameterStore StoreKind = "parameter-store"
)

// StoreInfo describes a config store extracted from a blueprint's celerity/config resource.
type StoreInfo struct {
	ResourceName  string
	StoreName     string
	StoreKind     StoreKind
	PlaintextKeys map[string]bool
}

// ConfigEntry represents a single config key/value with metadata.
type ConfigEntry struct {
	Key       string
	Value     string
	Sensitive bool
}

// ProviderOpts holds provider-specific options for creating a ConfigStore.
type ProviderOpts struct {
	Region  string
	Profile string
	Logger  *zap.Logger
}

package appconfig

import "context"

// ConfigStore provides CRUD operations for a cloud config/secret store.
// Each cloud provider implements this interface.
type ConfigStore interface {
	Get(ctx context.Context, key string) (*ConfigEntry, error)
	Set(ctx context.Context, key, value string, sensitive bool) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context) ([]ConfigEntry, error)
	Import(ctx context.Context, entries []ConfigEntry) error
}

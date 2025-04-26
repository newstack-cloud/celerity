package manage

// Entity provides a common interface for all entities
// modeled in the manage package.
type Entity interface {
	// GetID returns the ID of the entity.
	GetID() string
	// GetCreated returns the creation time of the entity
	// as a Unix timestamp in seconds.
	GetCreated() int64
}

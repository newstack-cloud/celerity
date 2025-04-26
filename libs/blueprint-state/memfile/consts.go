package memfile

const (
	// DefaultMaxGuideFileSize is the default maximum size of a state chunk file in bytes.
	DefaultMaxGuideFileSize = 1048576

	// DefaultMaxEventParititionSize is the default maximum size of an event partition in bytes.
	// When trying to save a new event that would exceed this size, the `memfile` state container
	// implementation will fail.
	DefaultMaxEventParititionSize = 10485760
)

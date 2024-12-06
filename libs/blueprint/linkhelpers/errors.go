package linkhelpers

import "errors"

var (
	// ErrMissingResolvedResource is an error that is returned when a set of resource
	// changes does not contain a resolved resource.
	ErrMissingResolvedResource = errors.New("resource changes missing resolved resource")
)

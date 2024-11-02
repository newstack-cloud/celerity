package state

// Error is an error type that MUST be used by implementations
// of the state.Container interface to allow the engine to be able to distinguish
// between failures and expected errors such as resource not found.
type Error struct {
	Code ErrorCode
}

type ErrorCode string

const (
	// ErrResourceNotFound is used when a resource could not be found
	// in a blueprint instance.
	ErrResourceNotFound ErrorCode = "resource_not_found"
	// ErrLinkNotFound is used when a link could not be found
	// in a blueprint instance.
	ErrLinkNotFound ErrorCode = "link_not_found"
	// ErrInstanceNotFound is used when a blueprint instance could not be found.
	ErrInstanceNotFound ErrorCode = "instance_not_found"
)

func (e *Error) Error() string {
	switch e.Code {
	case ErrResourceNotFound:
		return "StateError: resource not found"
	case ErrLinkNotFound:
		return "StateError: link not found"
	case ErrInstanceNotFound:
		return "StateError: instance not found"
	default:
		return "StateError: unknown error"
	}
}

// IsResourceNotFound is a helper function that checks if the provided error
// is a resource not found error.
func IsResourceNotFound(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == ErrResourceNotFound
	}
	return false
}

// IsLinkNotFound is a helper function that checks if the provided error
// is a link not found error.
func IsLinkNotFound(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == ErrLinkNotFound
	}
	return false
}

// IsInstanceNotFound is a helper function that checks if the provided error
// is an instance not found error.
func IsInstanceNotFound(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == ErrInstanceNotFound
	}
	return false
}

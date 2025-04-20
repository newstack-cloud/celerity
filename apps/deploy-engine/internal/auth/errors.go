package auth

import "fmt"

// Error is a type that represents an error
// that occurs during authentication.
// It is used to distinguish between authentication
// errors and other types of errors such as
// network or implementation errors.
type Error struct {
	ChildErr error
}

func (e *Error) Error() string {
	return fmt.Sprintf("authentication error: %v", e.ChildErr)
}

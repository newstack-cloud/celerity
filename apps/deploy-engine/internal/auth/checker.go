package auth

import (
	"context"
	"net/http"
)

// Checker provides an interface for carrying out
// an authentication check given a set of request
// headers.
type Checker interface {
	// Check carries out an authentication check
	// using the provided request headers.
	// It returns an error of the `auth.Error` type
	// if the check fails, or nil if it succeeds.
	// Other error types should be considered
	// as internal server errors.
	Check(ctx context.Context, headers http.Header) error
}

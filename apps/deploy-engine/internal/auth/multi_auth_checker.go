package auth

import (
	"context"
	"net/http"
)

type multiAuthChecker struct {
	checkers []Checker
}

// NewMultiAuthChecker creates a new instance of a Checker
// that combines multiple authenication methods
// where they are applied in the order that they are configured.
// As soon as a checker succeeds (returns nil instead of an error),
// any subsequent checkers are skipped.
// If all checkers fail, the last error is returned.
func NewMultiAuthChecker(checkers ...Checker) Checker {
	return &multiAuthChecker{
		checkers: checkers,
	}
}

func (m *multiAuthChecker) Check(ctx context.Context, headers http.Header) error {
	var lastErr error
	for _, checker := range m.checkers {
		err := checker.Check(ctx, headers)
		if err == nil {
			return nil
		}

		lastErr = err
	}

	return lastErr
}

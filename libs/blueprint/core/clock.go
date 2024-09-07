package core

import "time"

// Clock is an interface that provides the current time.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
}

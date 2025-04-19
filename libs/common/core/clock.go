package core

import "time"

// Clock is an interface that provides the current time.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
}

// SystemClock is a Clock that returns the current time
// derived from the host system.
type SystemClock struct{}

func (d SystemClock) Now() time.Time {
	return time.Now()
}

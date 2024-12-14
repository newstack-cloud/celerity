package core

import "time"

// Clock is an interface that provides the current time.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
	// Since returns the time elapsed since a given time.
	Since(time.Time) time.Duration
}

// SystemClock is a Clock that returns the current time
// derived from the host system.
type SystemClock struct{}

func (d SystemClock) Now() time.Time {
	return time.Now()
}

func (d SystemClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

// FractionalMilliseconds returns the fractional milliseconds of a given duration.
func FractionalMilliseconds(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

package manage

import "time"

const (
	// DefaultRecentlyQueuedEventsThreshold is the default threshold
	// for collecting recently queued events when a starting event ID
	// is not provided for a stream.
	DefaultRecentlyQueuedEventsThreshold = 300 * time.Second
)

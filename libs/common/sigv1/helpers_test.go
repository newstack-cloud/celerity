package sigv1

import "time"

const (
	// 2nd October 2024 19:00:52 UTC
	testTimestamp int64 = 1727895652
)

type testClock struct {
	timestamp int64
}

func (c *testClock) Now() time.Time {
	return time.Unix(c.timestamp, 0)
}

package testutils

import "time"

type MockClock struct {
	StaticTime   time.Time
	TimeSequence []time.Time
}

func (c *MockClock) Now() time.Time {
	if len(c.TimeSequence) > 0 {
		t := c.TimeSequence[0]
		c.TimeSequence = c.TimeSequence[1:]
		return t
	}
	return c.StaticTime
}

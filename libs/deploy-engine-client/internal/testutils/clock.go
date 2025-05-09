package testutils

import "time"

type MockClock struct {
	TimeSequence []time.Time
	currentIndex int
}

func (c *MockClock) Now() time.Time {
	if c.currentIndex >= len(c.TimeSequence) {
		c.currentIndex = 0
	}

	currentTime := c.TimeSequence[c.currentIndex]
	c.currentIndex += 1
	return currentTime
}

func (c *MockClock) Since(t time.Time) time.Duration {
	return c.TimeSequence[c.currentIndex].Sub(t)
}

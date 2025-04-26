package internal

import "time"

type MockClock struct {
	Timestamp int64
}

func (m *MockClock) Now() time.Time {
	return time.Unix(m.Timestamp, 0)
}

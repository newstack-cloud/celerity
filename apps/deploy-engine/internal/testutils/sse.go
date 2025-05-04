package testutils

import (
	"github.com/r3labs/sse/v2"
	"github.com/two-hundred/celerity/libs/blueprint-state/manage"
)

func SSEToManageEvent(
	event *sse.Event,
) *manage.Event {
	return &manage.Event{
		ID:   string(event.ID),
		Type: string(event.Event),
		Data: string(event.Data),
	}
}

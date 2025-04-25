package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint-state/manage"
)

func TestStreamEvents(
	fixtures []SaveEventFixture,
	events manage.Events,
	s *suite.Suite,
) {
	collectedEvents := []*manage.Event{}
	streamTo := make(chan manage.Event)
	errChan := make(chan error)
	endChan, err := events.Stream(
		context.Background(),
		&manage.EventStreamParams{
			ChannelType: "changesets",
			ChannelID:   "db58eda8-36c6-4180-a9cb-557f3392361c",
		},
		streamTo,
		errChan,
	)
	s.Require().NoError(err)

	go func() {
		for _, fixture := range fixtures {
			err := events.Save(
				context.Background(),
				fixture.Event,
			)
			if err != nil {
				fmt.Println("Failed to save event", err)
			}
		}
	}()

	// 3 existing events are streamed in addition to the new events
	// saved as a part of this test.
	totalToCollect := len(fixtures) + 3
	for len(collectedEvents) < totalToCollect && err == nil {
		select {
		case event := <-streamTo:
			collectedEvents = append(collectedEvents, &event)
		case err = <-errChan:
			s.Fail("Error in event stream", err)
		case <-time.After(20 * time.Second):
			s.Fail("Timeout waiting for event stream")
		}
	}

	s.Require().NoError(err)

	select {
	case endChan <- struct{}{}:
	case <-time.After(5 * time.Second):
		s.Fail("Timeout waiting for listener to handle end signal")
	}

	s.Assert().Len(collectedEvents, len(fixtures)+3)
	s.Assert().Equal(
		// Initial seed event 1 for the channel.
		"01966439-6832-74ba-94e3-9d8d47d98b60",
		collectedEvents[0].ID,
	)
	s.Assert().Equal(
		// Initial seed event 2 for the channel.
		"0196643a-69f6-7d6d-a4c1-c6ee239851a9",
		collectedEvents[1].ID,
	)
	s.Assert().Equal(
		// Initial seed event 3 for the channel.
		"0196643c-69b2-7900-bcf7-2ff34d80565e",
		collectedEvents[2].ID,
	)

	for i := 3; i < len(collectedEvents); i += 1 {
		// The deterministic value in the set of generated events is the data
		// field, which is a JSON encoded string of the generated event index.
		// The order of collected events will tell us if events were sent in the expected
		// order in the stream.
		generatedEventIndex := i - 3
		var actualData map[string]any
		err := json.Unmarshal([]byte(collectedEvents[i].Data), &actualData)
		s.Require().NoError(err)

		s.Assert().Equal(
			map[string]any{
				"value": fmt.Sprintf("%d", generatedEventIndex),
			},
			actualData,
		)
	}
}

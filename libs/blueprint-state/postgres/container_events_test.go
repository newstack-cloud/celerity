package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint-state/internal"
	"github.com/two-hundred/celerity/libs/blueprint-state/manage"
	"github.com/two-hundred/celerity/libs/blueprint/core"
)

const (
	nonExistentEventID        = "0196677d-d816-740c-8d99-457fee08eab1"
	cleanupThresholdTimestamp = 1743415200 // 31st Match 2025 10:00 UTC
)

type PostgresEventsTestSuite struct {
	container           *StateContainer
	connPool            *pgxpool.Pool
	saveEventFixtures   map[int]internal.SaveEventFixture
	streamEventFixtures []internal.SaveEventFixture
	suite.Suite
}

func (s *PostgresEventsTestSuite) SetupTest() {
	ctx := context.Background()
	connPool, err := pgxpool.New(ctx, buildTestDatabaseURL())
	s.connPool = connPool
	s.Require().NoError(err)
	container, err := LoadStateContainer(ctx, connPool, core.NewNopLogger())
	s.Require().NoError(err)
	s.container = container

	dirPath := path.Join("__testdata", "save-input", "events")
	saveFixtures, err := internal.SetupSaveEventFixtures(
		dirPath,
	)
	s.Require().NoError(err)
	s.saveEventFixtures = saveFixtures

	streamFixtures, err := createStreamSaveFixtures()
	s.Require().NoError(err)
	s.streamEventFixtures = streamFixtures
}

func (s *PostgresEventsTestSuite) TearDownTest() {
	s.connPool.Close()
}

func (s *PostgresEventsTestSuite) Test_saves_event_and_sends_notification() {
	fixture := s.saveEventFixtures[1]

	eventIDListener := make(chan string)
	go s.listenForEventNotification(eventIDListener, fixture.Event)

	// Sleep to ensure the listener is ready before saving the event
	// as the notification will not be received if the listener is not ready.
	time.Sleep(100 * time.Millisecond)

	events := s.container.Events()
	err := events.Save(
		context.Background(),
		fixture.Event,
	)
	s.Require().NoError(err)

	savedEvent, err := events.Get(
		context.Background(),
		fixture.Event.ID,
	)
	s.Require().NoError(err)
	s.Assert().NotNil(savedEvent)
	s.assertEventsEqual(fixture.Event, savedEvent)

	select {
	case eventID := <-eventIDListener:
		s.Assert().Equal(fixture.Event.ID, eventID)
	case <-time.After(10 * time.Second):
		s.Fail("Timeout waiting for event notification")
	}
}

func (s *PostgresEventsTestSuite) Test_stream_events() {
	collectedEvents := []*manage.Event{}
	streamTo := make(chan manage.Event)
	errChan := make(chan error)
	endChan, err := s.container.Events().Stream(
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
		for _, fixture := range s.streamEventFixtures {
			err := s.container.Events().Save(
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
	totalToCollect := len(s.streamEventFixtures) + 3
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

	s.Assert().Len(collectedEvents, len(s.streamEventFixtures)+3)
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
		s.Assert().Equal(
			fmt.Sprintf("{\"value\": \"%d\"}", generatedEventIndex),
			collectedEvents[i].Data,
		)
	}
}

func (s *PostgresEventsTestSuite) Test_returns_event_not_found_error_for_missing_event() {
	_, err := s.container.Events().Get(
		context.Background(),
		nonExistentEventID,
	)
	s.Require().Error(err)

	notFoundErr, isNotFoundErr := err.(*manage.EventNotFound)
	s.Require().True(isNotFoundErr)
	s.Assert().Equal(
		"event with ID 0196677d-d816-740c-8d99-457fee08eab1 not found",
		notFoundErr.Error(),
	)
}

func (s *PostgresEventsTestSuite) Test_cleans_up_old_events() {
	err := s.container.Events().Cleanup(
		context.Background(),
		time.Unix(cleanupThresholdTimestamp, 0),
	)
	s.Require().NoError(err)

	for _, id := range eventsShouldBeCleanedUp {
		_, err := s.container.Events().Get(
			context.Background(),
			id,
		)
		s.Require().Error(err)

		notFoundErr, isNotFoundErr := err.(*manage.EventNotFound)
		s.Require().True(isNotFoundErr)
		s.Assert().Equal(
			fmt.Sprintf("event with ID %s not found", id),
			notFoundErr.Error(),
		)
	}

	for _, id := range eventsShouldNotBeCleanedUp {
		event, err := s.container.Events().Get(
			context.Background(),
			id,
		)
		s.Require().NoError(err)
		s.Assert().Equal(id, event.ID)
	}
}

func (s *PostgresEventsTestSuite) listenForEventNotification(
	eventIDListener chan string,
	event *manage.Event,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		12*time.Second,
	)
	defer cancel()
	conn, err := s.connPool.Acquire(ctx)
	s.Require().NoError(err)
	channel := eventsChannel(event.ChannelType, event.ChannelID)
	_, err = conn.Conn().Exec(
		ctx,
		fmt.Sprintf("LISTEN %q", channel),
	)
	s.Require().NoError(err)
	defer s.unlistenForEventNotification(ctx, conn, channel)

	receivedEvent := false
	for !receivedEvent {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			s.Fail("Failed to wait for notification", err)
			return
		}
		if notification == nil {
			continue
		}

		receivedEvent = true
		eventID := notification.Payload
		eventIDListener <- eventID
	}
}

func (s *PostgresEventsTestSuite) unlistenForEventNotification(
	ctx context.Context,
	conn *pgxpool.Conn,
	channel string,
) {
	_, err := conn.Conn().Exec(
		ctx,
		fmt.Sprintf("UNLISTEN %q", channel),
	)
	if err != nil {
		fmt.Println("Failed to unlisten for event notification", err)
	}
	conn.Release()
}

func (s *PostgresEventsTestSuite) assertEventsEqual(
	expected *manage.Event,
	actual manage.Event,
) {
	s.Assert().Equal(expected.ID, actual.ID)
	s.Assert().Equal(expected.Type, actual.Type)
	s.Assert().Equal(expected.ChannelType, actual.ChannelType)
	s.Assert().Equal(expected.ChannelID, actual.ChannelID)

	if expected.Data != "" {
		var target map[string]any
		err := json.Unmarshal([]byte(expected.Data), &target)
		s.Require().NoError(err)

		var actualData map[string]any
		err = json.Unmarshal([]byte(actual.Data), &actualData)
		s.Require().NoError(err)

		s.Assert().Equal(target, actualData)
	}
	s.Assert().Equal(expected.Timestamp, actual.Timestamp)
}

func createStreamSaveFixtures() ([]internal.SaveEventFixture, error) {
	// Sleep between preparing each fixture to ensure the UUIDs contain different
	// timestamps to millisecond precision to assert that the events are
	// streamed in the correct order.
	fixtures := make([]internal.SaveEventFixture, len(streamFixtureEventIDs))
	for i := 0; i < len(streamFixtureEventIDs); i += 1 {
		id := streamFixtureEventIDs[i]

		fixtures[i] = internal.SaveEventFixture{
			Event: &manage.Event{
				ID:          id.String(),
				Type:        "resource",
				ChannelType: "changesets",
				ChannelID:   "db58eda8-36c6-4180-a9cb-557f3392361c",
				Data:        fmt.Sprintf("{\"value\":\"%d\"}", i),
				Timestamp:   time.Now().Unix(),
			},
		}
		time.Sleep(5 * time.Millisecond)
	}

	return fixtures, nil
}

// Seed events that should be cleaned up.
var eventsShouldBeCleanedUp = []string{
	"0196678c-ae43-7b53-9796-30e84ba07b99",
	"01966793-844a-7a2b-b278-48838bab3835",
	"01966794-8352-767e-a9d7-0ac6275af2e2",
	"01966794-f1e8-7a14-9893-335ca16be0d5",
	"01966795-8929-7f6d-989c-0403037d8131",
}

// Seed events that should not be cleaned up.
// This must not include the IDs of any dynamically generated events
// in the test runs.
var eventsShouldNotBeCleanedUp = []string{
	"01966439-6832-74ba-94e3-9d8d47d98b60",
	"01966439-ff85-760e-9f02-f3572e08a7c2",
	"0196643a-69f6-7d6d-a4c1-c6ee239851a9",
	"0196643c-69b2-7900-bcf7-2ff34d80565e",
}

// UUIDv7 values for event IDs in timestamp order.
var streamFixtureEventIDs = []uuid.UUID{
	uuid.MustParse("01966574-33ba-73c4-a5c0-a0b55249d39a"),
	uuid.MustParse("01966574-69ef-7b02-81cd-7fdbbbead77d"),
	uuid.MustParse("01966574-a5fe-7b22-9229-12a19afc8c32"),
	uuid.MustParse("01966575-47f8-7770-8a3c-56ea2e2b8dee"),
	uuid.MustParse("01966575-7ce6-7923-be83-011cebc8c8d3"),
	uuid.MustParse("01966575-a91e-7829-9f74-5069446071bf"),
	uuid.MustParse("01966576-0654-7f14-be3b-6af31cd6a1f5"),
	uuid.MustParse("01966576-368a-7a53-9f4e-38f9a5ef8ece"),
	uuid.MustParse("01966576-78b4-7711-9d4a-929e8dc29eb6"),
	uuid.MustParse("01966576-a6b5-7e37-8bf2-60e0eb10602e"),
	uuid.MustParse("01966576-e3e3-717d-bc25-324c29056a2f"),
	uuid.MustParse("01966577-3210-7562-8f1e-5a85200907b8"),
	uuid.MustParse("01966577-65f9-7cbb-ae74-93c7766c7d80"),
	uuid.MustParse("01966577-bff2-7829-b14b-7041be6c56b5"),
	uuid.MustParse("01966577-f76b-73b0-ae60-64d241ce4e8a"),
	uuid.MustParse("01966578-4544-729f-b968-b5893ea9fbdc"),
	uuid.MustParse("01966578-7675-7004-820f-d85b3e7616a7"),
	uuid.MustParse("01966578-acbf-735c-a318-6393dc267599"),
	uuid.MustParse("01966578-fe83-7cbd-8790-1a93dbf62e18"),
	uuid.MustParse("01966579-28d4-7c43-b4b2-f29238540587"),
}

func TestPostgresEventsTestSuite(t *testing.T) {
	suite.Run(t, new(PostgresEventsTestSuite))
}

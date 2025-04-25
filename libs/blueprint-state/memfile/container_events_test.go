package memfile

import (
	"context"
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint-state/internal"
	"github.com/two-hundred/celerity/libs/blueprint-state/manage"
	"github.com/two-hundred/celerity/libs/blueprint/core"
)

const (
	existingEventID           = "01966439-6832-74ba-94e3-9d8d47d98b60"
	nonExistentEventID        = "0196677d-d816-740c-8d99-457fee08eab1"
	cleanupThresholdTimestamp = 1743415200 // 31st Match 2025 10:00 UTC
)

type MemFileStateContainerEventsTestSuite struct {
	container           *StateContainer
	stateDir            string
	fs                  afero.Fs
	saveEventFixtures   map[int]internal.SaveEventFixture
	streamEventFixtures []internal.SaveEventFixture
	suite.Suite
}

func (s *MemFileStateContainerEventsTestSuite) SetupTest() {
	stateDir := path.Join("__testdata", "initial-state")
	memoryFS := afero.NewMemMapFs()
	loadMemoryFS(stateDir, memoryFS, &s.Suite)
	s.fs = memoryFS
	s.stateDir = stateDir
	// zapLogger, err := zap.NewDevelopment()
	// s.Require().NoError(err)
	// Use a low max partition file size to test reaching the partition
	// size limit.
	container, err := LoadStateContainer(
		stateDir,
		memoryFS,
		core.NewNopLogger(),
		WithMaxEventPartitionSize(1024), // 1KB
	)
	s.Require().NoError(err)
	s.container = container

	dirPath := path.Join("__testdata", "save-input", "events")
	fixtures, err := internal.SetupSaveEventFixtures(
		dirPath,
	)
	s.Require().NoError(err)
	s.saveEventFixtures = fixtures

	streamFixtures, err := internal.CreateEventStreamSaveFixtures()
	s.Require().NoError(err)
	s.streamEventFixtures = streamFixtures
}

func (s *MemFileStateContainerEventsTestSuite) Test_retrieves_event() {
	events := s.container.Events()
	event, err := events.Get(
		context.Background(),
		existingEventID,
	)
	s.Require().NoError(err)
	s.Require().NotNil(event)
	err = cupaloy.Snapshot(event)
	s.Require().NoError(err)
}

func (s *MemFileStateContainerEventsTestSuite) Test_reports_event_not_found_for_retrieval() {
	events := s.container.Events()

	_, err := events.Get(
		context.Background(),
		nonExistentEventID,
	)
	s.Require().Error(err)
	eventNotFoundErr, isEventNotFoundErr := err.(*manage.EventNotFound)
	s.Assert().True(isEventNotFoundErr)
	s.Assert().Equal(
		fmt.Sprintf("event with ID %s not found", nonExistentEventID),
		eventNotFoundErr.Error(),
	)
}

func (s *MemFileStateContainerEventsTestSuite) Test_saves_event() {
	fixture := s.saveEventFixtures[1]
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
	s.Assert().Equal(
		fixture.Event,
		&savedEvent,
	)
	s.assertPersistedEvent(fixture.Event)
}

func (s *MemFileStateContainerEventsTestSuite) Test_fails_to_save_event_that_pushes_partition_over_size_limit() {
	// Fixture 2 contains 8KB of serialised JSON data, which is
	// larger than the 1KB partition size limit.
	fixture := s.saveEventFixtures[2]
	events := s.container.Events()
	err := events.Save(
		context.Background(),
		fixture.Event,
	)
	s.Require().Error(err)
	memfileErr, isMemfileErr := err.(*Error)
	s.Require().True(isMemfileErr)
	s.Assert().Equal(
		ErrorReasonCodeMaxEventPartitionSizeExceeded,
		memfileErr.ReasonCode,
	)
}

func (s *MemFileStateContainerEventsTestSuite) Test_stream_events() {
	events := s.container.Events()
	internal.TestStreamEvents(
		s.streamEventFixtures,
		events,
		&s.Suite,
	)
}

func (s *MemFileStateContainerEventsTestSuite) Test_cleans_up_old_events() {
	err := s.container.Events().Cleanup(
		context.Background(),
		time.Unix(cleanupThresholdTimestamp, 0),
	)
	s.Require().NoError(err)

	assertEventsCleanedUp(
		s.container,
		&s.Suite,
	)

	// Assert that the events are cleaned up when loading a fresh
	// state container from file, ensuring that the cleanup
	// operation was persisted correctly.
	s.assertEventCleanupPersisted()
}

func (s *MemFileStateContainerEventsTestSuite) assertPersistedEvent(expected *manage.Event) {
	// Check that the event state was saved to "disk" correctly by
	// loading a new state container from persistence and retrieving the event.
	container, err := LoadStateContainer(s.stateDir, s.fs, core.NewNopLogger())
	s.Require().NoError(err)

	events := container.Events()
	persistedEvent, err := events.Get(
		context.Background(),
		expected.ID,
	)
	s.Require().NoError(err)
	s.Assert().Equal(
		expected,
		&persistedEvent,
	)
}

func (s *MemFileStateContainerEventsTestSuite) assertEventCleanupPersisted() {
	container, err := LoadStateContainer(s.stateDir, s.fs, core.NewNopLogger())
	s.Require().NoError(err)

	assertEventsCleanedUp(
		container,
		&s.Suite,
	)
}

func assertEventsCleanedUp(
	container *StateContainer,
	s *suite.Suite,
) {
	for _, id := range eventsShouldBeCleanedUp {
		_, err := container.Events().Get(
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
		event, err := container.Events().Get(
			context.Background(),
			id,
		)
		s.Require().NoError(err)
		s.Assert().Equal(id, event.ID)
	}
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

func TestMemFileStateContainerEventsTestSuite(t *testing.T) {
	suite.Run(t, new(MemFileStateContainerEventsTestSuite))
}

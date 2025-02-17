package memfile

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint-state/internal"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

const (
	existingLinkID    = "test-link-1"
	existingLinkName  = "saveOrderFunction::ordersTable_0"
	nonExistentLinkID = "non-existent-link"
)

type MemFileStateContainerLinksTestSuite struct {
	container        state.Container
	saveLinkFixtures map[int]saveLinkFixture
	stateDir         string
	fs               afero.Fs
	suite.Suite
}

func (s *MemFileStateContainerLinksTestSuite) SetupTest() {
	stateDir := path.Join("__testdata", "initial-state")
	memoryFS := afero.NewMemMapFs()
	loadMemoryFS(stateDir, memoryFS, &s.Suite)
	s.fs = memoryFS
	s.stateDir = stateDir
	// Use a low max guide file size of 100 bytes to trigger the logic that splits
	// instance and resource drift state across multiple chunk files.
	container, err := LoadStateContainer(stateDir, memoryFS, core.NewNopLogger(), WithMaxGuideFileSize(100))
	s.Require().NoError(err)
	s.container = container

	s.setupSaveLinkFixtures()
}

func (s *MemFileStateContainerLinksTestSuite) setupSaveLinkFixtures() {
	dirPath := path.Join("__testdata", "save-input", "links")
	dirEntries, err := os.ReadDir(dirPath)
	s.Require().NoError(err)

	s.saveLinkFixtures = make(map[int]saveLinkFixture)
	for i := 1; i <= len(dirEntries); i++ {
		fixture, err := loadSaveLinkFixture(i)
		s.Require().NoError(err)
		s.saveLinkFixtures[i] = fixture
	}
}

func (s *MemFileStateContainerLinksTestSuite) Test_retrieves_link() {
	links := s.container.Links()
	linkState, err := links.Get(
		context.Background(),
		existingLinkID,
	)
	s.Require().NoError(err)
	s.Require().NotNil(linkState)
	err = cupaloy.Snapshot(linkState)
	s.Require().NoError(err)
}

func (s *MemFileStateContainerLinksTestSuite) Test_reports_link_not_found_for_retrieval() {
	links := s.container.Links()

	_, err := links.Get(
		context.Background(),
		nonExistentLinkID,
	)
	s.Require().Error(err)
	stateErr, isStateErr := err.(*state.Error)
	s.Assert().True(isStateErr)
	s.Assert().Equal(state.ErrLinkNotFound, stateErr.Code)
}

func (s *MemFileStateContainerLinksTestSuite) Test_retrieves_link_by_logical_name() {
	links := s.container.Links()
	linkState, err := links.GetByName(
		context.Background(),
		existingBlueprintInstanceID,
		existingLinkName,
	)
	s.Require().NoError(err)
	s.Require().NotNil(linkState)
	err = cupaloy.Snapshot(linkState)
	s.Require().NoError(err)
}

func (s *MemFileStateContainerLinksTestSuite) Test_reports_link_not_found_for_retrieval_by_logical_name() {
	links := s.container.Links()

	_, err := links.GetByName(
		context.Background(),
		existingBlueprintInstanceID,
		nonExistentLinkID,
	)
	s.Require().Error(err)
	stateErr, isStateErr := err.(*state.Error)
	s.Assert().True(isStateErr)
	s.Assert().Equal(state.ErrLinkNotFound, stateErr.Code)
}

func (s *MemFileStateContainerLinksTestSuite) Test_saves_new_link() {
	fixture := s.saveLinkFixtures[1]
	links := s.container.Links()
	err := links.Save(
		context.Background(),
		*fixture.linkState,
	)
	s.Require().NoError(err)

	savedState, err := links.Get(
		context.Background(),
		fixture.linkState.LinkID,
	)
	s.Require().NoError(err)
	internal.AssertLinkStatesEqual(fixture.linkState, &savedState, &s.Suite)
	s.assertPersistedLink(fixture.linkState)
}

func (s *MemFileStateContainerLinksTestSuite) Test_reports_instance_not_found_for_saving_link() {
	// Fixture 2 is a link state that references a non-existent instance.
	fixture := s.saveLinkFixtures[2]
	links := s.container.Links()

	err := links.Save(
		context.Background(),
		*fixture.linkState,
	)
	s.Require().Error(err)
	stateErr, isStateErr := err.(*state.Error)
	s.Assert().True(isStateErr)
	s.Assert().Equal(state.ErrInstanceNotFound, stateErr.Code)
}

func (s *MemFileStateContainerLinksTestSuite) Test_updates_existing_link() {
	fixture := s.saveLinkFixtures[3]
	links := s.container.Links()
	err := links.Save(
		context.Background(),
		*fixture.linkState,
	)
	s.Require().NoError(err)

	savedState, err := links.Get(
		context.Background(),
		fixture.linkState.LinkID,
	)
	s.Require().NoError(err)
	internal.AssertLinkStatesEqual(fixture.linkState, &savedState, &s.Suite)
	s.assertPersistedLink(fixture.linkState)
}

func (s *MemFileStateContainerLinksTestSuite) Test_updates_blueprint_link_deployment_status() {
	links := s.container.Links()

	statusInfo := createTestLinkStatusInfo()
	err := links.UpdateStatus(
		context.Background(),
		existingLinkID,
		statusInfo,
	)
	s.Require().NoError(err)

	savedState, err := links.Get(
		context.Background(),
		existingLinkID,
	)
	s.Require().NoError(err)
	assertLinkStatusInfo(statusInfo, savedState, &s.Suite)
	s.assertPersistedLink(&savedState)
}

func (s *MemFileStateContainerLinksTestSuite) Test_reports_link_not_found_for_status_update() {
	links := s.container.Links()

	statusInfo := createTestLinkStatusInfo()
	err := links.UpdateStatus(
		context.Background(),
		nonExistentLinkID,
		statusInfo,
	)
	s.Require().Error(err)
	stateErr, isStateErr := err.(*state.Error)
	s.Assert().True(isStateErr)
	s.Assert().Equal(state.ErrLinkNotFound, stateErr.Code)
}

func (s *MemFileStateContainerLinksTestSuite) Test_reports_malformed_state_error_for_status_update() {
	// The malformed state for this test case contains a link
	// that references an instance that does not exist.
	container, err := loadMalformedStateContainer(&s.Suite)
	s.Require().NoError(err)

	links := container.Links()
	statusInfo := createTestLinkStatusInfo()
	err = links.UpdateStatus(
		context.Background(),
		existingLinkID,
		statusInfo,
	)
	s.Require().Error(err)
	memFileErr, isMemFileErr := err.(*Error)
	s.Assert().True(isMemFileErr)
	s.Assert().Equal(ErrorReasonCodeMalformedState, memFileErr.ReasonCode)
}

func (s *MemFileStateContainerLinksTestSuite) Test_removes_link() {
	links := s.container.Links()
	_, err := links.Remove(context.Background(), existingLinkID)
	s.Require().NoError(err)

	_, err = links.Get(context.Background(), existingLinkID)
	s.Require().Error(err)
	stateErr, isStateErr := err.(*state.Error)
	s.Assert().True(isStateErr)
	s.Assert().Equal(state.ErrLinkNotFound, stateErr.Code)

	s.assertLinkRemovedFromPersistence(existingResourceID)
}

func (s *MemFileStateContainerLinksTestSuite) Test_reports_link_not_found_for_removal() {
	links := s.container.Links()
	_, err := links.Remove(context.Background(), nonExistentLinkID)
	s.Require().Error(err)
	stateErr, isStateErr := err.(*state.Error)
	s.Assert().True(isStateErr)
	s.Assert().Equal(state.ErrLinkNotFound, stateErr.Code)
}

func (s *MemFileStateContainerLinksTestSuite) Test_reports_malformed_state_error_for_removal() {
	// The malformed state for this test case contains a link
	// that references an instance that does not exist.
	container, err := loadMalformedStateContainer(&s.Suite)
	s.Require().NoError(err)

	links := container.Links()
	_, err = links.Remove(
		context.Background(),
		existingLinkID,
	)
	s.Require().Error(err)
	memFileErr, isMemFileErr := err.(*Error)
	s.Assert().True(isMemFileErr)
	s.Assert().Equal(ErrorReasonCodeMalformedState, memFileErr.ReasonCode)
}

func (s *MemFileStateContainerLinksTestSuite) assertPersistedLink(expected *state.LinkState) {
	// Check that the link state was saved to "disk" correctly by
	// loading a new state container from persistence and retrieving the link.
	container, err := LoadStateContainer(s.stateDir, s.fs, core.NewNopLogger())
	s.Require().NoError(err)

	links := container.Links()
	savedLinkState, err := links.Get(
		context.Background(),
		expected.LinkID,
	)
	s.Require().NoError(err)
	internal.AssertLinkStatesEqual(expected, &savedLinkState, &s.Suite)
}

func (s *MemFileStateContainerLinksTestSuite) assertLinkRemovedFromPersistence(linkID string) {
	container, err := LoadStateContainer(s.stateDir, s.fs, core.NewNopLogger())
	s.Require().NoError(err)

	links := container.Links()
	_, err = links.Get(context.Background(), linkID)
	s.Require().Error(err)
	stateErr, isStateErr := err.(*state.Error)
	s.Assert().True(isStateErr)
	s.Assert().Equal(state.ErrLinkNotFound, stateErr.Code)
}

func assertLinkStatusInfo(
	expected state.LinkStatusInfo,
	actual state.LinkState,
	s *suite.Suite,
) {
	s.Assert().Equal(expected.Status, actual.Status)
	s.Assert().Equal(expected.PreciseStatus, actual.PreciseStatus)
	s.Assert().Equal(*expected.LastDeployedTimestamp, actual.LastDeployedTimestamp)
	s.Assert().Equal(*expected.LastDeployAttemptTimestamp, actual.LastDeployAttemptTimestamp)
	s.Assert().Equal(*expected.LastStatusUpdateTimestamp, actual.LastStatusUpdateTimestamp)
	s.Assert().Equal(expected.FailureReasons, actual.FailureReasons)
	s.Assert().Equal(expected.Durations, actual.Durations)
}

type saveLinkFixture struct {
	linkState *state.LinkState
}

func loadSaveLinkFixture(fixtureNumber int) (saveLinkFixture, error) {
	fileName := fmt.Sprintf("%d.json", fixtureNumber)
	filePath := path.Join("__testdata", "save-input", "links", fileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return saveLinkFixture{}, err
	}

	linkState := &state.LinkState{}
	err = json.Unmarshal(data, linkState)
	if err != nil {
		return saveLinkFixture{}, err
	}

	return saveLinkFixture{
		linkState: linkState,
	}, nil
}

func createTestLinkStatusInfo() state.LinkStatusInfo {
	lastDeployedTimestamp := 1739660528
	lastDeployAttemptTimestamp := 1739660528
	lastStatusUpdateTimestamp := 1739660528
	totalDuration := 2000.0
	resourceAAttemptDurations := []float64{2000.0}
	return state.LinkStatusInfo{
		Status:                     core.LinkStatusCreateFailed,
		LastDeployedTimestamp:      &lastDeployedTimestamp,
		LastDeployAttemptTimestamp: &lastDeployAttemptTimestamp,
		LastStatusUpdateTimestamp:  &lastStatusUpdateTimestamp,
		FailureReasons:             []string{"Failed to update resource A due to network error"},
		Durations: &state.LinkCompletionDurations{
			TotalDuration: &totalDuration,
			ResourceAUpdate: &state.LinkComponentCompletionDurations{
				AttemptDurations: resourceAAttemptDurations,
				TotalDuration:    &totalDuration,
			},
		},
	}
}

func TestMemFileStateContainerLinksTestSuite(t *testing.T) {
	suite.Run(t, new(MemFileStateContainerLinksTestSuite))
}

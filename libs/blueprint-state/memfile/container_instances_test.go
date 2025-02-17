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
	existingBlueprintInstanceID = "blueprint-instance-1"
	nonExistentInstanceID       = "non-existent-instance"
)

type MemFileStateContainerInstancesTestSuite struct {
	container             state.Container
	stateDir              string
	fs                    afero.Fs
	saveBlueprintFixtures map[int]saveBlueprintFixture
	suite.Suite
}

func (s *MemFileStateContainerInstancesTestSuite) SetupTest() {
	stateDir := path.Join("__testdata", "initial-state")
	memoryFS := afero.NewMemMapFs()
	loadMemoryFS(stateDir, memoryFS, &s.Suite)
	s.fs = memoryFS
	s.stateDir = stateDir
	// Use a low max guide file size of 100 bytes to trigger the logic that splits
	// instance state across multiple chunk files.
	container, err := LoadStateContainer(stateDir, memoryFS, core.NewNopLogger(), WithMaxGuideFileSize(100))
	s.Require().NoError(err)
	s.container = container

	s.setupSaveBlueprintFixtures()
}

func (s *MemFileStateContainerInstancesTestSuite) setupSaveBlueprintFixtures() {
	dirPath := path.Join("__testdata", "save-input", "blueprints")
	dirEntries, err := os.ReadDir(dirPath)
	s.Require().NoError(err)

	s.saveBlueprintFixtures = make(map[int]saveBlueprintFixture)
	for i := 1; i <= len(dirEntries); i++ {
		fixture, err := loadSaveBlueprintFixture(i)
		s.Require().NoError(err)
		s.saveBlueprintFixtures[i] = fixture
	}
}

func (s *MemFileStateContainerInstancesTestSuite) Test_retrieves_instance() {
	instances := s.container.Instances()
	instanceState, err := instances.Get(
		context.Background(),
		existingBlueprintInstanceID,
	)
	s.Require().NoError(err)
	s.Require().NotNil(instanceState)
	err = cupaloy.Snapshot(instanceState)
	s.Require().NoError(err)
}

func (s *MemFileStateContainerInstancesTestSuite) Test_reports_instance_not_found_for_retrieval() {
	instances := s.container.Instances()

	_, err := instances.Get(
		context.Background(),
		nonExistentInstanceID,
	)
	s.Require().Error(err)
	stateErr, isStateErr := err.(*state.Error)
	s.Assert().True(isStateErr)
	s.Assert().Equal(state.ErrInstanceNotFound, stateErr.Code)
}

func (s *MemFileStateContainerInstancesTestSuite) Test_saves_new_instance_with_child_blueprint() {
	fixture := s.saveBlueprintFixtures[1]
	instances := s.container.Instances()
	err := instances.Save(
		context.Background(),
		*fixture.instanceState,
	)
	s.Require().NoError(err)

	savedState, err := instances.Get(
		context.Background(),
		fixture.instanceState.InstanceID,
	)
	s.Require().NoError(err)
	internal.AssertInstanceStatesEqual(fixture.instanceState, &savedState, &s.Suite)
	s.assertPersistedInstance(fixture.instanceState)
}

func (s *MemFileStateContainerInstancesTestSuite) Test_updates_existing_instance_with_child_blueprint() {
	fixture := s.saveBlueprintFixtures[2]
	instances := s.container.Instances()
	err := instances.Save(
		context.Background(),
		*fixture.instanceState,
	)
	s.Require().NoError(err)

	savedState, err := instances.Get(
		context.Background(),
		fixture.instanceState.InstanceID,
	)
	s.Require().NoError(err)
	internal.AssertInstanceStatesEqual(fixture.instanceState, &savedState, &s.Suite)
	s.assertPersistedInstance(fixture.instanceState)
}

func (s *MemFileStateContainerInstancesTestSuite) Test_updates_blueprint_instance_deployment_status() {
	instances := s.container.Instances()

	statusInfo := createTestInstanceStatusInfo()
	err := instances.UpdateStatus(
		context.Background(),
		existingBlueprintInstanceID,
		statusInfo,
	)
	s.Require().NoError(err)

	savedState, err := instances.Get(
		context.Background(),
		existingBlueprintInstanceID,
	)
	s.Require().NoError(err)
	assertInstanceStatusInfo(statusInfo, savedState, &s.Suite)
	s.assertPersistedInstance(&savedState)
}

func (s *MemFileStateContainerInstancesTestSuite) Test_reports_instance_not_found_for_status_update() {
	instances := s.container.Instances()

	statusInfo := createTestInstanceStatusInfo()
	err := instances.UpdateStatus(
		context.Background(),
		nonExistentInstanceID,
		statusInfo,
	)
	s.Require().Error(err)
	stateErr, isStateErr := err.(*state.Error)
	s.Assert().True(isStateErr)
	s.Assert().Equal(state.ErrInstanceNotFound, stateErr.Code)
}

func (s *MemFileStateContainerInstancesTestSuite) Test_removes_blueprint_instance() {
	instances := s.container.Instances()
	_, err := instances.Remove(context.Background(), existingBlueprintInstanceID)
	s.Require().NoError(err)

	_, err = instances.Get(context.Background(), existingBlueprintInstanceID)
	s.Require().Error(err)
	stateErr, isStateErr := err.(*state.Error)
	s.Assert().True(isStateErr)
	s.Assert().Equal(state.ErrInstanceNotFound, stateErr.Code)

	s.assertInstanceRemovedFromPersistence(existingBlueprintInstanceID)
}

func (s *MemFileStateContainerInstancesTestSuite) Test_reports_instance_not_found_for_removal() {
	instances := s.container.Instances()
	_, err := instances.Remove(context.Background(), nonExistentInstanceID)
	s.Require().Error(err)
	stateErr, isStateErr := err.(*state.Error)
	s.Assert().True(isStateErr)
	s.Assert().Equal(state.ErrInstanceNotFound, stateErr.Code)
}

func (s *MemFileStateContainerInstancesTestSuite) assertPersistedInstance(expected *state.InstanceState) {
	// Check that the instance state was saved to "disk" correctly by
	// loading a new state container from persistence and retrieving the instance.
	container, err := LoadStateContainer(s.stateDir, s.fs, core.NewNopLogger())
	s.Require().NoError(err)

	instances := container.Instances()
	savedInstanceState, err := instances.Get(
		context.Background(),
		expected.InstanceID,
	)
	s.Require().NoError(err)
	internal.AssertInstanceStatesEqual(expected, &savedInstanceState, &s.Suite)
}

func (s *MemFileStateContainerInstancesTestSuite) assertInstanceRemovedFromPersistence(instanceID string) {
	container, err := LoadStateContainer(s.stateDir, s.fs, core.NewNopLogger())
	s.Require().NoError(err)

	instances := container.Instances()
	_, err = instances.Get(context.Background(), instanceID)
	s.Require().Error(err)
	stateErr, isStateErr := err.(*state.Error)
	s.Assert().True(isStateErr)
	s.Assert().Equal(state.ErrInstanceNotFound, stateErr.Code)
}

func assertInstanceStatusInfo(
	expected state.InstanceStatusInfo,
	actual state.InstanceState,
	s *suite.Suite,
) {
	s.Assert().Equal(expected.Status, actual.Status)
	s.Assert().Equal(*expected.LastDeployedTimestamp, actual.LastDeployedTimestamp)
	s.Assert().Equal(*expected.LastDeployAttemptTimestamp, actual.LastDeployAttemptTimestamp)
	s.Assert().Equal(*expected.LastStatusUpdateTimestamp, actual.LastStatusUpdateTimestamp)
	s.Assert().Equal(expected.Durations, actual.Durations)
}

type saveBlueprintFixture struct {
	instanceState *state.InstanceState
}

func loadSaveBlueprintFixture(fixtureNumber int) (saveBlueprintFixture, error) {
	fileName := fmt.Sprintf("%d.json", fixtureNumber)
	filePath := path.Join("__testdata", "save-input", "blueprints", fileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return saveBlueprintFixture{}, err
	}

	instanceState := &state.InstanceState{}
	err = json.Unmarshal(data, instanceState)
	if err != nil {
		return saveBlueprintFixture{}, err
	}

	return saveBlueprintFixture{
		instanceState: instanceState,
	}, nil
}

func createTestInstanceStatusInfo() state.InstanceStatusInfo {
	lastDeployedTimestamp := 1739660528
	lastDeployAttemptTimestamp := 1739660528
	lastStatusUpdateTimestamp := 1739660528
	prepareDuration := 1000.0
	totalDuration := 2000.0
	return state.InstanceStatusInfo{
		Status:                     core.InstanceStatusDeployFailed,
		LastDeployedTimestamp:      &lastDeployedTimestamp,
		LastDeployAttemptTimestamp: &lastDeployAttemptTimestamp,
		LastStatusUpdateTimestamp:  &lastStatusUpdateTimestamp,
		Durations: &state.InstanceCompletionDuration{
			PrepareDuration: &prepareDuration,
			TotalDuration:   &totalDuration,
		},
	}
}

func TestMemFileStateContainerInstancesTestSuite(t *testing.T) {
	suite.Run(t, new(MemFileStateContainerInstancesTestSuite))
}

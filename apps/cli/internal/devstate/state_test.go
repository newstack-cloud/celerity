package devstate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type StateTestSuite struct {
	suite.Suite
}

func (s *StateTestSuite) Test_Write_and_Load_roundtrip() {
	dir := s.T().TempDir()

	state := &DevState{
		ContainerID:    "abc123",
		ContainerName:  "celerity-dev-myapp",
		ComposeProject: "celerity-dev-myapp",
		Image:          "ghcr.io/newstack-cloud/celerity-runtime-nodejs-22:dev-0.3.1",
		HostPort:       "8080",
		AppDir:         dir,
		BlueprintFile:  filepath.Join(dir, ".celerity", "merged.blueprint.yaml"),
		ServiceName:    "myapp",
		Handlers: []HandlerSummary{
			{Name: "Hello", Type: "http", Method: "GET", Path: "/hello"},
			{Name: "Orders", Type: "http", Method: "POST", Path: "/orders"},
		},
		StartedAt: time.Now().Truncate(time.Second),
		PID:       12345,
		Detached:  false,
	}

	err := Write(dir, state)
	s.Require().NoError(err)

	loaded, err := Load(dir)
	s.Require().NoError(err)
	s.Require().NotNil(loaded)

	s.Assert().Equal(stateVersion, loaded.Version)
	s.Assert().Equal("abc123", loaded.ContainerID)
	s.Assert().Equal("celerity-dev-myapp", loaded.ContainerName)
	s.Assert().Equal("8080", loaded.HostPort)
	s.Assert().Len(loaded.Handlers, 2)
	s.Assert().Equal("Hello", loaded.Handlers[0].Name)
	s.Assert().Equal(12345, loaded.PID)
	s.Assert().False(loaded.Detached)
}

func (s *StateTestSuite) Test_Load_returns_nil_when_not_found() {
	dir := s.T().TempDir()

	loaded, err := Load(dir)
	s.Require().NoError(err)
	s.Assert().Nil(loaded)
}

func (s *StateTestSuite) Test_Remove_deletes_state_file() {
	dir := s.T().TempDir()

	state := &DevState{ContainerID: "abc"}
	s.Require().NoError(Write(dir, state))

	s.Require().NoError(Remove(dir))

	loaded, err := Load(dir)
	s.Require().NoError(err)
	s.Assert().Nil(loaded)
}

func (s *StateTestSuite) Test_Remove_no_error_when_absent() {
	dir := s.T().TempDir()
	err := Remove(dir)
	s.Assert().NoError(err)
}

func (s *StateTestSuite) Test_Write_creates_celerity_directory() {
	dir := s.T().TempDir()
	state := &DevState{ContainerID: "abc"}
	s.Require().NoError(Write(dir, state))

	_, err := os.Stat(filepath.Join(dir, ".celerity", "dev.state.json"))
	s.Assert().NoError(err)
}

func (s *StateTestSuite) Test_IsProcessAlive_returns_true_for_current_process() {
	state := &DevState{PID: os.Getpid()}
	s.Assert().True(state.IsProcessAlive())
}

func (s *StateTestSuite) Test_IsProcessAlive_returns_false_for_zero_pid() {
	state := &DevState{PID: 0}
	s.Assert().False(state.IsProcessAlive())
}

func (s *StateTestSuite) Test_IsProcessAlive_returns_false_for_dead_pid() {
	// PID 99999999 is extremely unlikely to exist.
	state := &DevState{PID: 99999999}
	s.Assert().False(state.IsProcessAlive())
}

func TestStateTestSuite(t *testing.T) {
	suite.Run(t, new(StateTestSuite))
}

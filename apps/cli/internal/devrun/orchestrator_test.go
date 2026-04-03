package devrun

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/devstate"
	"github.com/newstack-cloud/celerity/apps/cli/internal/docker"
	"github.com/newstack-cloud/celerity/apps/cli/internal/testutils"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type OrchestratorTestSuite struct {
	suite.Suite
	logger *zap.Logger
}

func TestOrchestratorTestSuite(t *testing.T) {
	suite.Run(t, new(OrchestratorTestSuite))
}

func (s *OrchestratorTestSuite) SetupTest() {
	logger, _ := zap.NewDevelopment()
	s.logger = logger
}

func (s *OrchestratorTestSuite) writeState(appDir string, state *devstate.DevState) {
	stateDir := filepath.Join(appDir, ".celerity")
	s.Require().NoError(os.MkdirAll(stateDir, 0o755))
	data, err := json.Marshal(state)
	s.Require().NoError(err)
	s.Require().NoError(os.WriteFile(filepath.Join(stateDir, "dev.state.json"), data, 0o644))
}

func (s *OrchestratorTestSuite) Test_load_state_for_command_returns_state() {
	appDir := s.T().TempDir()
	s.writeState(appDir, &devstate.DevState{
		Version:       1,
		ContainerID:   "abc123",
		ContainerName: "my-app",
		HostPort:      "8080",
	})

	state, err := LoadStateForCommand(appDir)
	s.Require().NoError(err)
	s.Assert().Equal("abc123", state.ContainerID)
	s.Assert().Equal("my-app", state.ContainerName)
}

func (s *OrchestratorTestSuite) Test_load_state_for_command_no_state_returns_error() {
	appDir := s.T().TempDir()
	_, err := LoadStateForCommand(appDir)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "no dev environment running")
}

func (s *OrchestratorTestSuite) Test_handle_stale_state_no_state_is_noop() {
	appDir := s.T().TempDir()
	var buf bytes.Buffer
	output := NewOutput(&buf, false)
	mockDocker := &testutils.MockDockerManager{}

	err := HandleStaleState(context.Background(), appDir, mockDocker, nil, output)
	s.Require().NoError(err)
	s.Assert().Empty(mockDocker.CleanupStaleCalls)
}

func (s *OrchestratorTestSuite) Test_handle_stale_state_cleans_up_stale_container() {
	appDir := s.T().TempDir()
	s.writeState(appDir, &devstate.DevState{
		Version:       1,
		ContainerID:   "abc123",
		ContainerName: "my-app",
		PID:           999999999, // Non-existent PID, so IsProcessAlive returns false.
	})

	var buf bytes.Buffer
	output := NewOutput(&buf, false)
	mockDocker := &testutils.MockDockerManager{}

	err := HandleStaleState(context.Background(), appDir, mockDocker, nil, output)
	s.Require().NoError(err)
	s.Assert().Contains(mockDocker.CleanupStaleCalls, "my-app")
	s.Assert().Contains(buf.String(), "Cleaning up stale")
}

func (s *OrchestratorTestSuite) Test_shutdown_stops_container_and_removes_state() {
	appDir := s.T().TempDir()
	s.writeState(appDir, &devstate.DevState{Version: 1})

	var buf bytes.Buffer
	output := NewOutput(&buf, false)
	mockDocker := &testutils.MockDockerManager{}

	orch := &Orchestrator{
		config: OrchestratorConfig{
			AppDir: appDir,
			ContainerCfg: &docker.ContainerConfig{
				ContainerName: "test-container",
			},
		},
		docker:      mockDocker,
		output:      output,
		logger:      s.logger,
		containerID: "container-123",
	}

	err := orch.Shutdown(context.Background())
	s.Require().NoError(err)
	s.Assert().Contains(mockDocker.StopCalls, "container-123")
	s.Assert().Contains(buf.String(), "Dev environment stopped")

	// State file should be removed.
	_, loadErr := devstate.Load(appDir)
	s.Require().NoError(loadErr) // Load returns nil,nil when no file.
}

func (s *OrchestratorTestSuite) Test_shutdown_no_container_id_skips_stop() {
	appDir := s.T().TempDir()

	var buf bytes.Buffer
	output := NewOutput(&buf, false)
	mockDocker := &testutils.MockDockerManager{}

	orch := &Orchestrator{
		config: OrchestratorConfig{
			AppDir:       appDir,
			ContainerCfg: &docker.ContainerConfig{},
		},
		docker: mockDocker,
		output: output,
		logger: s.logger,
	}

	err := orch.Shutdown(context.Background())
	s.Require().NoError(err)
	s.Assert().Empty(mockDocker.StopCalls)
}

func (s *OrchestratorTestSuite) Test_stop_from_state_no_state_prints_no_environment() {
	appDir := s.T().TempDir()
	var buf bytes.Buffer
	output := NewOutput(&buf, false)

	err := StopFromState(context.Background(), appDir, &testutils.MockDockerManager{}, nil, output, s.logger)
	s.Require().NoError(err)
	s.Assert().Contains(buf.String(), "No dev environment running")
}

func (s *OrchestratorTestSuite) Test_stop_from_state_shuts_down() {
	appDir := s.T().TempDir()
	s.writeState(appDir, &devstate.DevState{
		Version:       1,
		ContainerID:   "abc123",
		ContainerName: "test-app",
		ServiceName:   "myapp",
	})

	var buf bytes.Buffer
	output := NewOutput(&buf, false)
	mockDocker := &testutils.MockDockerManager{}

	err := StopFromState(context.Background(), appDir, mockDocker, nil, output, s.logger)
	s.Require().NoError(err)
	s.Assert().Contains(mockDocker.StopCalls, "abc123")
	s.Assert().Contains(buf.String(), "Stopping dev environment")
	s.Assert().Contains(buf.String(), "Dev environment stopped")
}

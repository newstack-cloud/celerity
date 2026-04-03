package devrun

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/newstack-cloud/celerity/apps/cli/internal/blueprint"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devstate"
	"github.com/stretchr/testify/suite"
)

type OutputTestSuite struct {
	suite.Suite
}

func TestOutputTestSuite(t *testing.T) {
	suite.Run(t, new(OutputTestSuite))
}

func (s *OutputTestSuite) Test_print_step_with_color() {
	var buf bytes.Buffer
	o := NewOutput(&buf, true)
	o.PrintStep("done")
	out := buf.String()
	s.Assert().Contains(out, "done")
	s.Assert().Contains(out, colorGreen)
}

func (s *OutputTestSuite) Test_print_step_without_color() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	o.PrintStep("done")
	out := buf.String()
	s.Assert().Contains(out, "done")
	s.Assert().NotContains(out, colorGreen)
}

func (s *OutputTestSuite) Test_print_error() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	o.PrintError("failed", errors.New("boom"))
	out := buf.String()
	s.Assert().Contains(out, "failed")
	s.Assert().Contains(out, "boom")
}

func (s *OutputTestSuite) Test_print_warning() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	o.PrintWarning("watch out", errors.New("issue"))
	out := buf.String()
	s.Assert().Contains(out, "watch out")
	s.Assert().Contains(out, "issue")
}

func (s *OutputTestSuite) Test_print_progress() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	o.PrintProgress("loading...")
	s.Assert().Contains(buf.String(), "loading...")
}

func (s *OutputTestSuite) Test_print_info() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	o.PrintInfo("info msg")
	s.Assert().Contains(buf.String(), "info msg")
}

func (s *OutputTestSuite) Test_writer_returns_underlying_writer() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	s.Assert().Equal(&buf, o.Writer())
}

func (s *OutputTestSuite) Test_print_startup_summary_with_handlers() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	handlers := []blueprint.HandlerInfo{
		{Method: "GET", Path: "/users", HandlerName: "getUsers"},
		{Method: "POST", Path: "/users", HandlerName: "createUser"},
	}
	o.PrintStartupSummary("8080", handlers)
	out := buf.String()
	s.Assert().Contains(out, "http://localhost:8080")
	s.Assert().Contains(out, "GET")
	s.Assert().Contains(out, "/users")
	s.Assert().Contains(out, "getUsers")
	s.Assert().Contains(out, "POST")
}

func (s *OutputTestSuite) Test_print_startup_summary_without_handlers() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	o.PrintStartupSummary("3000", nil)
	out := buf.String()
	s.Assert().Contains(out, "http://localhost:3000")
	s.Assert().NotContains(out, "Handlers:")
}

func (s *OutputTestSuite) Test_print_streaming_notice() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	o.PrintStreamingNotice()
	s.Assert().Contains(buf.String(), "Streaming logs")
}

func (s *OutputTestSuite) Test_print_detached_notice() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	o.PrintDetachedNotice()
	out := buf.String()
	s.Assert().Contains(out, "background")
	s.Assert().Contains(out, "celerity dev logs")
	s.Assert().Contains(out, "celerity dev stop")
}

func (s *OutputTestSuite) Test_print_shutdown_starting_and_complete() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	o.PrintShutdownStarting()
	o.PrintShutdownComplete()
	out := buf.String()
	s.Assert().Contains(out, "Stopping dev environment")
	s.Assert().Contains(out, "Dev environment stopped")
}

func (s *OutputTestSuite) Test_print_status_running_foreground() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	state := &devstate.DevState{
		StartedAt:     time.Now().Add(-5 * time.Minute),
		ContainerName: "my-app",
		ContainerID:   "abc123def456789",
		Image:         "celerity-runtime:latest",
		HostPort:      "8080",
		PID:           12345,
	}
	o.PrintStatus(state, true)
	out := buf.String()
	s.Assert().Contains(out, "Running")
	s.Assert().Contains(out, "my-app")
	s.Assert().Contains(out, "abc123def456")
	s.Assert().Contains(out, "8080")
	s.Assert().Contains(out, "foreground (PID 12345)")
}

func (s *OutputTestSuite) Test_print_status_stale_detached() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	state := &devstate.DevState{
		StartedAt:     time.Now().Add(-1 * time.Hour),
		ContainerName: "my-app",
		ContainerID:   "short",
		Image:         "img",
		HostPort:      "3000",
		Detached:      true,
	}
	o.PrintStatus(state, false)
	out := buf.String()
	s.Assert().Contains(out, "Stale")
	s.Assert().Contains(out, "detached")
}

func (s *OutputTestSuite) Test_print_no_environment() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	o.PrintNoEnvironment()
	s.Assert().Contains(buf.String(), "No dev environment running")
}

func (s *OutputTestSuite) Test_print_test_header() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	o.PrintTestHeader([]string{"unit", "integration"})
	s.Assert().Contains(buf.String(), "unit, integration")
}

func (s *OutputTestSuite) Test_print_test_passed() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	o.PrintTestPassed()
	s.Assert().Contains(buf.String(), "Tests passed")
}

func (s *OutputTestSuite) Test_print_test_failed() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	o.PrintTestFailed(2)
	out := buf.String()
	s.Assert().Contains(out, "Tests failed")
	s.Assert().Contains(out, "exit code 2")
}

func (s *OutputTestSuite) Test_print_health_waiting_and_ready() {
	var buf bytes.Buffer
	o := NewOutput(&buf, false)
	o.PrintHealthWaiting()
	o.PrintHealthReady(2500 * time.Millisecond)
	out := buf.String()
	s.Assert().Contains(out, "Waiting for app")
	s.Assert().Contains(out, "App ready")
	s.Assert().Contains(out, "2.5s")
}

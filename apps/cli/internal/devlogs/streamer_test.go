package devlogs

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/docker"
	"github.com/stretchr/testify/suite"
)

// mockDockerManager is a minimal mock that returns pre-canned log data.
type mockDockerManager struct {
	logData string
}

func (m *mockDockerManager) CheckAvailability(ctx context.Context) error { return nil }
func (m *mockDockerManager) EnsureImage(ctx context.Context, image string, progress chan<- docker.ImagePullProgress) error {
	return nil
}
func (m *mockDockerManager) CreateAndStart(ctx context.Context, config *docker.ContainerConfig) (string, error) {
	return "", nil
}
func (m *mockDockerManager) StreamLogs(ctx context.Context, containerID string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(m.logData)), nil
}
func (m *mockDockerManager) StreamLogsWithOptions(ctx context.Context, containerID string, opts docker.LogStreamOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(m.logData)), nil
}
func (m *mockDockerManager) RestartContainer(ctx context.Context, containerID string) error {
	return nil
}
func (m *mockDockerManager) Stop(ctx context.Context, containerID string) error { return nil }
func (m *mockDockerManager) IsRunning(ctx context.Context, containerID string) (bool, error) {
	return true, nil
}
func (m *mockDockerManager) CleanupStale(ctx context.Context, containerName string) error {
	return nil
}

type StreamerTestSuite struct {
	suite.Suite
}

func (s *StreamerTestSuite) Test_pino_json_still_formatted() {
	input := `{"level":30,"time":"2024-01-01T00:00:00Z","msg":"order created","handlerName":"Orders"}` + "\n"

	out := s.stream(input, StreamOptions{})
	// The formatter wraps handler name in brackets.
	s.Assert().Contains(out, "[Orders")
	s.Assert().Contains(out, "order created")
}

func (s *StreamerTestSuite) Test_pino_json_parsed_with_versioned_runtime() {
	// Runtimes are stored as "nodejs24.x" — ensure pino JSON is parsed with a versioned runtime.
	input := `{"level":30,"time":"2024-01-01T00:00:00Z","msg":"user created","handlerName":"Users","name":"request"}` + "\n"

	out := s.streamWithRuntime(input, StreamOptions{}, "nodejs24.x")
	s.Assert().Contains(out, "[Users")
	s.Assert().Contains(out, "user created")
	// pino "name" field should not leak into output as raw JSON.
	s.Assert().NotContains(out, `"name"`)
}

func (s *StreamerTestSuite) stream(input string, opts StreamOptions) string {
	return s.streamWithRuntime(input, opts, "nodejs")
}

func (s *StreamerTestSuite) streamWithRuntime(input string, opts StreamOptions, runtime string) string {
	mock := &mockDockerManager{logData: input}
	streamer := NewStreamer(mock, runtime, false)
	var buf bytes.Buffer
	err := streamer.Stream(context.Background(), "test-container", opts, &buf)
	s.Require().NoError(err)
	return buf.String()
}

func TestStreamerTestSuite(t *testing.T) {
	suite.Run(t, new(StreamerTestSuite))
}

package testutils

import (
	"context"
	"io"
	"strings"

	"github.com/newstack-cloud/celerity/apps/cli/internal/docker"
)

// MockDockerManager is a test double for docker.RuntimeContainerManager.
// Set the function fields to control behaviour; unset fields return nil/zero.
type MockDockerManager struct {
	CheckAvailabilityFn func(ctx context.Context) error
	EnsureImageFn       func(ctx context.Context, image string, progress chan<- docker.ImagePullProgress) error
	CreateAndStartFn    func(ctx context.Context, config *docker.ContainerConfig) (string, error)
	StreamLogsFn        func(ctx context.Context, containerID string) (io.ReadCloser, error)
	StreamLogsOptsFn    func(ctx context.Context, containerID string, opts docker.LogStreamOptions) (io.ReadCloser, error)
	RestartContainerFn  func(ctx context.Context, containerID string) error
	StopFn              func(ctx context.Context, containerID string) error
	IsRunningFn         func(ctx context.Context, containerID string) (bool, error)
	CleanupStaleFn      func(ctx context.Context, containerName string) error

	// Call records for assertions.
	StopCalls         []string
	CleanupStaleCalls []string
}

func (m *MockDockerManager) CheckAvailability(ctx context.Context) error {
	if m.CheckAvailabilityFn != nil {
		return m.CheckAvailabilityFn(ctx)
	}
	return nil
}

func (m *MockDockerManager) EnsureImage(ctx context.Context, image string, progress chan<- docker.ImagePullProgress) error {
	if m.EnsureImageFn != nil {
		return m.EnsureImageFn(ctx, image, progress)
	}
	if progress != nil {
		close(progress)
	}
	return nil
}

func (m *MockDockerManager) CreateAndStart(ctx context.Context, config *docker.ContainerConfig) (string, error) {
	if m.CreateAndStartFn != nil {
		return m.CreateAndStartFn(ctx, config)
	}
	return "mock-container-id", nil
}

func (m *MockDockerManager) StreamLogs(ctx context.Context, containerID string) (io.ReadCloser, error) {
	if m.StreamLogsFn != nil {
		return m.StreamLogsFn(ctx, containerID)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *MockDockerManager) StreamLogsWithOptions(ctx context.Context, containerID string, opts docker.LogStreamOptions) (io.ReadCloser, error) {
	if m.StreamLogsOptsFn != nil {
		return m.StreamLogsOptsFn(ctx, containerID, opts)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *MockDockerManager) RestartContainer(ctx context.Context, containerID string) error {
	if m.RestartContainerFn != nil {
		return m.RestartContainerFn(ctx, containerID)
	}
	return nil
}

func (m *MockDockerManager) Stop(ctx context.Context, containerID string) error {
	m.StopCalls = append(m.StopCalls, containerID)
	if m.StopFn != nil {
		return m.StopFn(ctx, containerID)
	}
	return nil
}

func (m *MockDockerManager) IsRunning(ctx context.Context, containerID string) (bool, error) {
	if m.IsRunningFn != nil {
		return m.IsRunningFn(ctx, containerID)
	}
	return false, nil
}

func (m *MockDockerManager) CleanupStale(ctx context.Context, containerName string) error {
	m.CleanupStaleCalls = append(m.CleanupStaleCalls, containerName)
	if m.CleanupStaleFn != nil {
		return m.CleanupStaleFn(ctx, containerName)
	}
	return nil
}

package docker

import (
	"context"
	"io"
)

// RuntimeContainerManager manages Docker containers for the Celerity dev runtime.
type RuntimeContainerManager interface {
	CheckAvailability(ctx context.Context) error
	EnsureImage(ctx context.Context, image string, progress chan<- ImagePullProgress) error
	CreateAndStart(ctx context.Context, config *ContainerConfig) (string, error)
	StreamLogs(ctx context.Context, containerID string) (io.ReadCloser, error)
	StreamLogsWithOptions(ctx context.Context, containerID string, opts LogStreamOptions) (io.ReadCloser, error)
	RestartContainer(ctx context.Context, containerID string) error
	Stop(ctx context.Context, containerID string) error
	IsRunning(ctx context.Context, containerID string) (bool, error)
	CleanupStale(ctx context.Context, containerName string) error
}

// ContainerConfig holds the configuration for creating a runtime container.
type ContainerConfig struct {
	Image         string
	ContainerName string
	Cmd           []string
	HostPort      string
	ContainerPort string
	AppDir        string
	EnvVars       map[string]string
	Binds         []string
	NetworkName   string
}

// LogStreamOptions configures container log streaming with filtering support.
type LogStreamOptions struct {
	Follow     bool
	Tail       string // "all" or a number like "100"
	Since      string // RFC3339 timestamp or relative duration
	Timestamps bool
}

// ImagePullProgress reports progress of a Docker image pull operation.
type ImagePullProgress struct {
	ID       string
	Status   string
	Progress string
	Error    string
}

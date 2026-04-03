package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"go.uber.org/zap"
)

const (
	stopTimeout    = 10 * time.Second
	restartTimeout = 10 * time.Second
)

// RuntimeContainer implements RuntimeContainerManager using the Docker Engine API.
type RuntimeContainer struct {
	client *client.Client
	logger *zap.Logger
}

// NewRuntimeContainer creates a new Docker client for managing runtime containers.
func NewRuntimeContainer(logger *zap.Logger) (*RuntimeContainer, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating Docker client: %w", err)
	}

	return &RuntimeContainer{
		client: cli,
		logger: logger,
	}, nil
}

func (rc *RuntimeContainer) CheckAvailability(ctx context.Context) error {
	_, err := rc.client.Ping(ctx)
	if err != nil {
		return fmt.Errorf(
			"docker daemon is not running or not accessible: %w", err,
		)
	}
	return nil
}

func (rc *RuntimeContainer) EnsureImage(
	ctx context.Context,
	img string,
	progress chan<- ImagePullProgress,
) error {
	defer close(progress)

	_, _, err := rc.client.ImageInspectWithRaw(ctx, img)
	if err == nil {
		rc.logger.Debug("image already present locally", zap.String("image", img))
		return nil
	}

	rc.logger.Info("pulling image", zap.String("image", img))
	reader, err := rc.client.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image %s: %w", img, err)
	}
	defer reader.Close()

	return decodeImagePullProgress(reader, progress)
}

func (rc *RuntimeContainer) CreateAndStart(
	ctx context.Context,
	cfg *ContainerConfig,
) (string, error) {
	containerCfg := buildContainerConfig(cfg)
	hostCfg := buildHostConfig(cfg)

	resp, err := rc.client.ContainerCreate(
		ctx, containerCfg, hostCfg, nil, nil, cfg.ContainerName,
	)
	if err != nil {
		return "", fmt.Errorf("creating container %s: %w", cfg.ContainerName, err)
	}

	if err := rc.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("starting container %s: %w", cfg.ContainerName, err)
	}

	rc.logger.Info("container started",
		zap.String("id", resp.ID[:12]),
		zap.String("name", cfg.ContainerName),
	)
	return resp.ID, nil
}

func (rc *RuntimeContainer) StreamLogs(
	ctx context.Context,
	containerID string,
) (io.ReadCloser, error) {
	logReader, err := rc.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	})
	if err != nil {
		return nil, fmt.Errorf("streaming logs for container %s: %w", containerID[:12], err)
	}

	// Docker multiplexes stdout/stderr with an 8-byte header per frame.
	// Demux into a single stream via stdcopy.
	pr, pw := io.Pipe()
	go func() {
		_, err := stdcopy.StdCopy(pw, pw, logReader)
		logReader.Close()
		pw.CloseWithError(err)
	}()

	return pr, nil
}

func (rc *RuntimeContainer) StreamLogsWithOptions(
	ctx context.Context,
	containerID string,
	opts LogStreamOptions,
) (io.ReadCloser, error) {
	tail := opts.Tail
	if tail == "" {
		tail = "all"
	}

	logReader, err := rc.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     opts.Follow,
		Timestamps: opts.Timestamps,
		Tail:       tail,
		Since:      opts.Since,
	})
	if err != nil {
		return nil, fmt.Errorf("streaming logs for container %s: %w", containerID[:12], err)
	}

	pr, pw := io.Pipe()
	go func() {
		_, err := stdcopy.StdCopy(pw, pw, logReader)
		logReader.Close()
		pw.CloseWithError(err)
	}()

	return pr, nil
}

func (rc *RuntimeContainer) RestartContainer(
	ctx context.Context,
	containerID string,
) error {
	timeout := int(restartTimeout.Seconds())
	if err := rc.client.ContainerRestart(
		ctx, containerID, container.StopOptions{Timeout: &timeout},
	); err != nil {
		return fmt.Errorf("restarting container %s: %w", containerID[:12], err)
	}
	return nil
}

func (rc *RuntimeContainer) Stop(ctx context.Context, containerID string) error {
	timeout := int(stopTimeout.Seconds())
	if err := rc.client.ContainerStop(
		ctx, containerID, container.StopOptions{Timeout: &timeout},
	); err != nil {
		rc.logger.Warn("error stopping container",
			zap.String("id", containerID[:12]),
			zap.Error(err),
		)
	}

	if err := rc.client.ContainerRemove(
		ctx, containerID, container.RemoveOptions{Force: true},
	); err != nil {
		return fmt.Errorf("removing container %s: %w", containerID[:12], err)
	}

	return nil
}

func (rc *RuntimeContainer) IsRunning(ctx context.Context, containerID string) (bool, error) {
	info, err := rc.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, nil
	}
	return info.State != nil && info.State.Running, nil
}

func (rc *RuntimeContainer) CleanupStale(
	ctx context.Context,
	containerName string,
) error {
	info, err := rc.client.ContainerInspect(ctx, containerName)
	if err != nil {
		// Container doesn't exist — nothing to clean up.
		return nil
	}

	rc.logger.Info("cleaning up stale container",
		zap.String("name", containerName),
		zap.String("id", info.ID[:12]),
	)
	return rc.Stop(ctx, info.ID)
}

func buildContainerConfig(cfg *ContainerConfig) *container.Config {
	env := make([]string, 0, len(cfg.EnvVars))
	for k, v := range cfg.EnvVars {
		env = append(env, k+"="+v)
	}

	exposedPort := nat.Port(cfg.ContainerPort + "/tcp")

	return &container.Config{
		Image:        cfg.Image,
		Cmd:          cfg.Cmd,
		Env:          env,
		ExposedPorts: nat.PortSet{exposedPort: struct{}{}},
	}
}

func buildHostConfig(cfg *ContainerConfig) *container.HostConfig {
	exposedPort := nat.Port(cfg.ContainerPort + "/tcp")
	hostCfg := &container.HostConfig{
		Binds: cfg.Binds,
		PortBindings: nat.PortMap{
			exposedPort: []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: cfg.HostPort},
			},
		},
	}

	if cfg.NetworkName != "" {
		hostCfg.NetworkMode = container.NetworkMode(cfg.NetworkName)
	}

	return hostCfg
}

// pullProgressEvent matches the JSON structure from Docker's image pull stream.
type pullProgressEvent struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	Progress       string `json:"progress"`
	ProgressDetail struct {
		Current int64 `json:"current"`
		Total   int64 `json:"total"`
	} `json:"progressDetail"`
	ErrorMessage string `json:"error"`
}

func decodeImagePullProgress(
	reader io.Reader,
	progress chan<- ImagePullProgress,
) error {
	decoder := json.NewDecoder(reader)
	for decoder.More() {
		var event pullProgressEvent
		if err := decoder.Decode(&event); err != nil {
			return fmt.Errorf("decoding pull progress: %w", err)
		}

		if event.ErrorMessage != "" {
			return fmt.Errorf("pull error: %s", event.ErrorMessage)
		}

		progress <- ImagePullProgress{
			ID:       event.ID,
			Status:   event.Status,
			Progress: event.Progress,
		}
	}
	return nil
}

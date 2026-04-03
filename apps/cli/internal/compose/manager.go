package compose

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// ComposeManager wraps the Docker Compose CLI for managing the local dependency stack.
type ComposeManager struct {
	dockerBin     string
	projectName   string
	generatedFile string
	overrideFile  string
	logger        *zap.Logger
}

// NewComposeManager creates a compose manager for the given project.
// appDir is the root directory of the Celerity app.
func NewComposeManager(projectName string, appDir string, logger *zap.Logger) (*ComposeManager, error) {
	dockerBin, err := exec.LookPath("docker")
	if err != nil {
		return nil, fmt.Errorf("docker not found on PATH: %w", err)
	}

	celerityDir := filepath.Join(appDir, ".celerity")
	overridePath := filepath.Join(appDir, "compose.yaml")

	return &ComposeManager{
		dockerBin:     dockerBin,
		projectName:   projectName,
		generatedFile: filepath.Join(celerityDir, "compose.generated.yaml"),
		overrideFile:  overridePath,
		logger:        logger,
	}, nil
}

// Up starts the compose stack with health check waiting.
// When output is non-nil, compose stderr is streamed through in real time.
func (cm *ComposeManager) Up(ctx context.Context, output io.Writer) error {
	args := cm.baseArgs()
	args = append(args, "up", "-d", "--wait")
	if output != nil {
		return cm.runWithOutput(ctx, args, output)
	}
	return cm.run(ctx, args)
}

// Down tears down the compose stack.
func (cm *ComposeManager) Down(ctx context.Context) error {
	args := cm.baseArgs()
	args = append(args, "down")
	return cm.run(ctx, args)
}

// IsRunning checks if the compose stack is already up.
func (cm *ComposeManager) IsRunning(ctx context.Context) bool {
	args := cm.baseArgs()
	args = append(args, "ps", "--status", "running", "-q")

	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, cm.dockerBin, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return false
	}

	return strings.TrimSpace(stdout.String()) != ""
}

// NetworkName returns the Docker network name created by compose.
// Docker Compose names networks as <project>_default.
func (cm *ComposeManager) NetworkName() string {
	return cm.projectName + "_default"
}

// Logs fetches the logs from a specific service in the compose stack.
// It returns the combined stdout/stderr output as a string.
// If tail > 0, only the last N lines are returned.
func (cm *ComposeManager) Logs(ctx context.Context, service string, tail int) (string, error) {
	args := cm.baseArgs()
	args = append(args, "logs", "--no-color")
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}
	args = append(args, service)

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, cm.dockerBin, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker compose logs failed: %w\n%s", err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}

// UnhealthyServices returns the names of services that are in an unhealthy state.
func (cm *ComposeManager) UnhealthyServices(ctx context.Context) []string {
	args := cm.baseArgs()
	args = append(args, "ps", "--status", "unhealthy", "--format", "{{.Service}}")

	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, cm.dockerBin, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		cm.logger.Debug("failed to list unhealthy services", zap.Error(err))
		return nil
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return nil
	}

	return strings.Split(output, "\n")
}

// HasServices returns true if the generated compose file exists and
// contains at least one service.
func (cm *ComposeManager) HasServices() bool {
	info, err := os.Stat(cm.generatedFile)
	return err == nil && info.Size() > 0
}

func (cm *ComposeManager) baseArgs() []string {
	args := []string{
		"compose",
		"-p", cm.projectName,
		"-f", cm.generatedFile,
	}

	if _, err := os.Stat(cm.overrideFile); err == nil {
		args = append(args, "-f", cm.overrideFile)
	}

	return args
}

func (cm *ComposeManager) runWithOutput(ctx context.Context, args []string, output io.Writer) error {
	cm.logger.Debug("running docker compose", zap.Strings("args", args))

	cmd := exec.CommandContext(ctx, cm.dockerBin, args...)
	cmd.Stderr = output
	cmd.Stdout = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(
			"docker %s failed: %w",
			strings.Join(args[1:], " "),
			err,
		)
	}

	return nil
}

func (cm *ComposeManager) run(ctx context.Context, args []string) error {
	cm.logger.Debug("running docker compose", zap.Strings("args", args))

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, cm.dockerBin, args...)
	cmd.Stderr = &stderr
	cmd.Stdout = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(
			"docker %s failed: %w\n%s",
			strings.Join(args[1:], " "),
			err,
			strings.TrimSpace(stderr.String()),
		)
	}

	return nil
}

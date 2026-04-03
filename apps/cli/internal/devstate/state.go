package devstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const (
	stateVersion = 1
	stateDir     = ".celerity"
	stateFile    = "dev.state.json"
)

// DevState tracks the running dev environment for coordination between
// dev run, dev stop, dev status, and dev logs commands.
type DevState struct {
	Version        int              `json:"version"`
	ContainerID    string           `json:"containerId"`
	ContainerName  string           `json:"containerName"`
	ComposeProject string           `json:"composeProject"`
	Image          string           `json:"image"`
	HostPort       string           `json:"hostPort"`
	AppDir         string           `json:"appDir"`
	BlueprintFile  string           `json:"blueprintFile"`
	ServiceName    string           `json:"serviceName"`
	Runtime        string           `json:"runtime"`
	Handlers       []HandlerSummary `json:"handlers"`
	StartedAt      time.Time        `json:"startedAt"`
	PID            int              `json:"pid"`
	Detached       bool             `json:"detached"`
}

// HandlerSummary is a minimal handler description for status display.
type HandlerSummary struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`
}

// StatePath returns the full path to the state file for the given app directory.
func StatePath(appDir string) string {
	return filepath.Join(appDir, stateDir, stateFile)
}

// Write atomically writes the state file using a temp file + rename.
func Write(appDir string, state *DevState) error {
	state.Version = stateVersion
	dir := filepath.Join(appDir, stateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}

	path := StatePath(appDir)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming state file: %w", err)
	}

	return nil
}

// Load reads and parses the state file. Returns nil if not found.
func Load(appDir string) (*DevState, error) {
	data, err := os.ReadFile(StatePath(appDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var state DevState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}

	if state.Version != stateVersion {
		return nil, fmt.Errorf(
			"state file version %d not supported (expected %d), delete %s and retry",
			state.Version, stateVersion, StatePath(appDir),
		)
	}

	return &state, nil
}

// Remove deletes the state file. No error if already absent.
func Remove(appDir string) error {
	err := os.Remove(StatePath(appDir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing state file: %w", err)
	}
	return nil
}

// IsProcessAlive checks whether the PID recorded in the state is still running.
// Returns false for PID 0 (detached mode) or if the process is gone.
func (s *DevState) IsProcessAlive() bool {
	if s.PID <= 0 {
		return false
	}

	proc, err := os.FindProcess(s.PID)
	if err != nil {
		return false
	}

	// Signal 0 checks process existence without affecting it.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

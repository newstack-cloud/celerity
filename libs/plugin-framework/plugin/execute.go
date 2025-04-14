package plugin

import (
	"os"
	"os/exec"
)

// PluginExecutor is an interface that represents the executor of a plugin.
// This interface is used to abstract the execution of a plugin from the
// plugin launcher.
type PluginExecutor interface {
	// Execute the plugin binary at the given path.
	Execute(pluginBinary string) (PluginProcess, error)
}

// PluginProcess is an interface that represents a running plugin process.
type PluginProcess interface {
	// Kill the plugin process.
	Kill() error
}

type osCmdExecutor struct{}

// NewOSCmdExecutor creates a new PluginExecutor that uses an
// operating system command to execute the plugin binary.
func NewOSCmdExecutor() PluginExecutor {
	return &osCmdExecutor{}
}

func (e *osCmdExecutor) Execute(pluginBinary string) (PluginProcess, error) {
	cmd := exec.Command(pluginBinary)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	return &osCmdProcess{cmd}, nil
}

type osCmdProcess struct {
	cmd *exec.Cmd
}

func (p *osCmdProcess) Kill() error {
	return p.cmd.Process.Kill()
}

package plugin

import (
	"os"
	"os/exec"
	"path"
	"strings"
)

// PluginExecutor is an interface that represents the executor of a plugin.
// This interface is used to abstract the execution of a plugin from the
// plugin launcher.
type PluginExecutor interface {
	// Execute the plugin binary at the given path.
	Execute(pluginID string, pluginBinary string) (PluginProcess, error)
}

// PluginProcess is an interface that represents a running plugin process.
type PluginProcess interface {
	// Kill the plugin process.
	Kill() error
}

type osCmdExecutor struct {
	logFileRootDir string
	env            map[string]string
}

// NewOSCmdExecutor creates a new PluginExecutor that uses an
// operating system command to execute the plugin binary.
// stdout and stderr for each plugin will be redirected to a log file
// for the plugin under the logFileRootDir directory.
// The log file will be located at:
// {logFileRootDir}/({pluginHost}/?)/{namespace}/{pluginName}/plugin.log
func NewOSCmdExecutor(
	logFileRootDir string,
	env map[string]string,
) PluginExecutor {
	return &osCmdExecutor{
		logFileRootDir: logFileRootDir,
		env:            env,
	}
}

func (e *osCmdExecutor) Execute(
	pluginID string,
	pluginBinary string,
) (PluginProcess, error) {
	cmd := exec.Command(pluginBinary)
	cmd.Env = os.Environ()
	cmd.Env = addEnvVars(cmd.Env, e.env)
	pluginLogFile, err := e.openLogFile(pluginID)
	if err != nil {
		return nil, err
	}
	cmd.Stdout = pluginLogFile
	cmd.Stderr = pluginLogFile
	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	return &osCmdProcess{cmd}, nil
}

func (e *osCmdExecutor) openLogFile(pluginID string) (*os.File, error) {
	pluginIDSegments := strings.Split(pluginID, "/")
	pathSegments := append(
		[]string{
			e.logFileRootDir,
		},
		pluginIDSegments...,
	)
	pluginLogDir := path.Join(
		pathSegments...,
	)
	err := os.MkdirAll(pluginLogDir, 0755)
	if err != nil {
		return nil, err
	}

	pluginAbsPath := path.Join(
		pluginLogDir,
		"plugin.log",
	)

	return os.OpenFile(pluginAbsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

func addEnvVars(
	env []string,
	newEnv map[string]string,
) []string {
	for k, v := range newEnv {
		env = append(env, k+"="+v)
	}
	return env
}

type osCmdProcess struct {
	cmd *exec.Cmd
}

func (p *osCmdProcess) Kill() error {
	return p.cmd.Process.Kill()
}

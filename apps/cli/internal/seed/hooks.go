package seed

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"go.uber.org/zap"
)

// HookLine represents a single line of output from a hook script.
type HookLine struct {
	Script string
	Line   string
	IsErr  bool
}

// HookOutputFunc is called for each line of output from a hook script.
type HookOutputFunc func(line HookLine)

// RunHooks executes post-setup hook scripts from the seed directory.
// Each script receives service endpoint environment variables so hooks
// can interact with running local services. Output is streamed line-by-line
// through the onOutput callback.
func RunHooks(
	ctx context.Context,
	scripts []string,
	serviceEndpoints map[string]string,
	onOutput HookOutputFunc,
	logger *zap.Logger,
) error {
	if len(scripts) == 0 {
		return nil
	}

	env := buildHookEnv(serviceEndpoints)

	for _, script := range scripts {
		if err := runHook(ctx, script, env, onOutput, logger); err != nil {
			return err
		}
	}

	return nil
}

func runHook(
	ctx context.Context,
	script string,
	env []string,
	onOutput HookOutputFunc,
	logger *zap.Logger,
) error {
	scriptName := filepath.Base(script)
	logger.Debug("running seed hook", zap.String("script", scriptName))

	if err := checkExecutable(script); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, script)
	cmd.Env = env

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe for %s: %w", scriptName, err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("creating stderr pipe for %s: %w", scriptName, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting hook %s: %w", scriptName, err)
	}

	// Stream both stdout and stderr concurrently.
	done := make(chan struct{}, 2)
	go streamLines(stdoutPipe, scriptName, false, onOutput, done)
	go streamLines(stderrPipe, scriptName, true, onOutput, done)

	// Wait for both readers to finish before calling cmd.Wait().
	<-done
	<-done

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("hook %s failed: %w", scriptName, err)
	}

	return nil
}

func streamLines(
	r io.Reader,
	scriptName string,
	isErr bool,
	onOutput HookOutputFunc,
	done chan<- struct{},
) {
	defer func() { done <- struct{}{} }()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		onOutput(HookLine{
			Script: scriptName,
			Line:   scanner.Text(),
			IsErr:  isErr,
		})
	}
}

func buildHookEnv(serviceEndpoints map[string]string) []string {
	// Start with the current process environment so hooks have access
	// to PATH and other standard variables.
	env := os.Environ()
	for key, value := range serviceEndpoints {
		env = append(env, key+"="+value)
	}
	return env
}

func checkExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("hook script not found: %w", err)
	}

	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		return fmt.Errorf(
			"hook script %s is not executable — run: chmod +x %s",
			filepath.Base(path), path,
		)
	}
	return nil
}

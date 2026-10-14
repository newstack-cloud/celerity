package handlertoolchain

import (
	"context"
	"io"
	"strings"
)

// recordedCommand is one invocation a stubRunner saw.
type recordedCommand struct {
	Dir  string
	Env  map[string]string
	Name string
	Args []string
}

func (c recordedCommand) line() string {
	return strings.Join(append([]string{c.Name}, c.Args...), " ")
}

// stubRunner records commands instead of running them, and can be told to
// fail one or to write a file as a side effect of another.
type stubRunner struct {
	commands []recordedCommand
	// failOn fails any command whose rendered line contains this substring.
	failOn string
	err    error
	// output is what Output returns.
	output []byte
	// onRun runs after a command is recorded, to stand in for whatever the
	// real command would have written to disk.
	onRun func(c recordedCommand) error
}

func (r *stubRunner) record(dir string, env map[string]string, name string, args []string) recordedCommand {
	c := recordedCommand{Dir: dir, Env: env, Name: name, Args: args}
	r.commands = append(r.commands, c)
	return c
}

func (r *stubRunner) check(c recordedCommand) error {
	if r.failOn != "" && strings.Contains(c.line(), r.failOn) {
		if r.err != nil {
			return r.err
		}
		return io.ErrUnexpectedEOF
	}
	if r.onRun != nil {
		return r.onRun(c)
	}
	return nil
}

func (r *stubRunner) Run(
	_ context.Context, dir string, env map[string]string, _ io.Writer, name string, args ...string,
) error {
	return r.check(r.record(dir, env, name, args))
}

func (r *stubRunner) Output(
	_ context.Context, dir string, env map[string]string, _ io.Writer, name string, args ...string,
) ([]byte, error) {
	if err := r.check(r.record(dir, env, name, args)); err != nil {
		return nil, err
	}
	return r.output, nil
}

// lines renders every recorded command for assertions.
func (r *stubRunner) lines() []string {
	out := make([]string, 0, len(r.commands))
	for _, c := range r.commands {
		out = append(out, c.line())
	}
	return out
}

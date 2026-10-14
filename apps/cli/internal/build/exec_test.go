package build

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// stubExec records all command invocations without executing them.
// It writes a dummy file to the target directory so zips aren't empty.
type stubExec struct {
	calls []execCall
}

type execCall struct {
	dir  string
	name string
	args []string
}

func (s *stubExec) Run(_ context.Context, dir string, _ io.Writer, name string, args ...string) error {
	s.calls = append(s.calls, execCall{dir: dir, name: name, args: args})
	// Write a dummy file so downstream zip creation has something to archive.
	if dir != "" {
		_ = os.MkdirAll(dir, 0755)
		_ = os.WriteFile(filepath.Join(dir, "stub-output"), []byte("stub"), 0644)
	}
	return nil
}

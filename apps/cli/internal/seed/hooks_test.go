package seed

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type HooksTestSuite struct {
	suite.Suite
	logger *zap.Logger
}

func TestHooksTestSuite(t *testing.T) {
	suite.Run(t, new(HooksTestSuite))
}

func (s *HooksTestSuite) SetupTest() {
	logger, _ := zap.NewDevelopment()
	s.logger = logger
}

func (s *HooksTestSuite) Test_run_hooks_empty_scripts_returns_nil() {
	err := RunHooks(context.Background(), nil, nil, nil, s.logger)
	s.Assert().NoError(err)
}

func (s *HooksTestSuite) Test_run_hooks_executes_script_and_captures_output() {
	if runtime.GOOS == "windows" {
		s.T().Skip("skipping on windows")
	}

	dir := s.T().TempDir()
	script := filepath.Join(dir, "hook.sh")
	err := os.WriteFile(script, []byte("#!/bin/sh\necho hello\n"), 0o755)
	s.Require().NoError(err)

	var lines []HookLine
	onOutput := func(line HookLine) {
		lines = append(lines, line)
	}

	err = RunHooks(context.Background(), []string{script}, map[string]string{"FOO": "bar"}, onOutput, s.logger)
	s.Require().NoError(err)
	s.Require().NotEmpty(lines)
	s.Assert().Equal("hello", lines[0].Line)
	s.Assert().Equal("hook.sh", lines[0].Script)
	s.Assert().False(lines[0].IsErr)
}

func (s *HooksTestSuite) Test_run_hooks_nonexecutable_script_returns_error() {
	if runtime.GOOS == "windows" {
		s.T().Skip("skipping on windows")
	}

	dir := s.T().TempDir()
	script := filepath.Join(dir, "hook.sh")
	err := os.WriteFile(script, []byte("#!/bin/sh\necho hi\n"), 0o644) // not executable
	s.Require().NoError(err)

	err = RunHooks(context.Background(), []string{script}, nil, func(HookLine) {}, s.logger)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "not executable")
}

func (s *HooksTestSuite) Test_run_hooks_missing_script_returns_error() {
	err := RunHooks(context.Background(), []string{"/nonexistent/hook.sh"}, nil, func(HookLine) {}, s.logger)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "not found")
}

func (s *HooksTestSuite) Test_run_hooks_failing_script_returns_error() {
	if runtime.GOOS == "windows" {
		s.T().Skip("skipping on windows")
	}

	dir := s.T().TempDir()
	script := filepath.Join(dir, "fail.sh")
	err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755)
	s.Require().NoError(err)

	err = RunHooks(context.Background(), []string{script}, nil, func(HookLine) {}, s.logger)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "failed")
}

func (s *HooksTestSuite) Test_run_hooks_passes_service_endpoints_as_env() {
	if runtime.GOOS == "windows" {
		s.T().Skip("skipping on windows")
	}

	dir := s.T().TempDir()
	outFile := filepath.Join(dir, "out.txt")
	script := filepath.Join(dir, "check-env.sh")
	err := os.WriteFile(script, []byte("#!/bin/sh\necho $MY_ENDPOINT > "+outFile+"\n"), 0o755)
	s.Require().NoError(err)

	endpoints := map[string]string{"MY_ENDPOINT": "http://localhost:9000"}
	err = RunHooks(context.Background(), []string{script}, endpoints, func(HookLine) {}, s.logger)
	s.Require().NoError(err)

	data, err := os.ReadFile(outFile)
	s.Require().NoError(err)
	s.Assert().Contains(string(data), "http://localhost:9000")
}

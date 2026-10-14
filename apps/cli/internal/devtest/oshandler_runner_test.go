package devtest

import (
	"context"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// fakeToolchain answers with the test commands it was given.
type fakeToolchain struct {
	commands map[string][]string
}

func (t *fakeToolchain) Name() string          { return "fake" }
func (t *fakeToolchain) Validate(string) error { return nil }

func (t *fakeToolchain) Prepare(context.Context, handlertoolchain.PrepareParams) error { return nil }

func (t *fakeToolchain) Extract(
	context.Context, handlertoolchain.ExtractParams,
) (*preprocess.HandlerManifest, error) {
	return nil, nil
}

func (t *fakeToolchain) BuildBinary(context.Context, handlertoolchain.BinaryParams) error {
	return nil
}

func (t *fakeToolchain) WatchPaths() handlertoolchain.WatchPaths {
	return handlertoolchain.WatchPaths{}
}

func (t *fakeToolchain) TestCommand(suite string) ([]string, bool) {
	command, ok := t.commands[suite]
	return command, ok
}

type OSHandlerRunnerSuite struct {
	suite.Suite
	logger *zap.Logger
}

func (s *OSHandlerRunnerSuite) SetupTest() { s.logger = zap.NewNop() }

func (s *OSHandlerRunnerSuite) config(suites ...TestSuite) RunConfig {
	return RunConfig{
		AppDir:  s.T().TempDir(),
		Runtime: "os-only.debian13",
		Suites:  suites,
	}
}

func (s *OSHandlerRunnerSuite) Test_runs_the_toolchains_command_for_the_suite() {
	tc := &fakeToolchain{commands: map[string][]string{"unit": {"true"}}}
	runner := NewOSHandlerRunner(tc, s.logger)

	result, err := runner.Run(context.Background(), s.config(SuiteUnit))
	s.Require().NoError(err)
	s.Assert().Zero(result.ExitCode)
}

// A failing suite has to fail the run, or a red test suite reads as green.
func (s *OSHandlerRunnerSuite) Test_a_failing_suite_fails_the_run() {
	tc := &fakeToolchain{commands: map[string][]string{"unit": {"false"}}}
	runner := NewOSHandlerRunner(tc, s.logger)

	result, err := runner.Run(context.Background(), s.config(SuiteUnit))
	s.Require().NoError(err)
	s.Assert().NotZero(result.ExitCode)
}

// The worst outcome wins: a later passing suite must not mask an earlier
// failure.
func (s *OSHandlerRunnerSuite) Test_one_failing_suite_among_several_fails_the_run() {
	tc := &fakeToolchain{commands: map[string][]string{
		"unit":        {"false"},
		"integration": {"true"},
	}}
	runner := NewOSHandlerRunner(tc, s.logger)

	result, err := runner.Run(context.Background(), s.config(SuiteUnit, SuiteIntegration))
	s.Require().NoError(err)
	s.Assert().NotZero(result.ExitCode)
}

func (s *OSHandlerRunnerSuite) Test_an_explicit_test_command_wins() {
	tc := &fakeToolchain{commands: map[string][]string{"unit": {"false"}}}
	runner := NewOSHandlerRunner(tc, s.logger)

	config := s.config(SuiteUnit)
	config.TestCommand = "true"

	result, err := runner.Run(context.Background(), config)
	s.Require().NoError(err)
	s.Assert().Zero(result.ExitCode)
}

func (s *OSHandlerRunnerSuite) Test_no_command_for_any_suite_says_how_to_declare_one() {
	runner := NewOSHandlerRunner(&fakeToolchain{}, s.logger)

	_, err := runner.Run(context.Background(), s.config(SuiteUnit))
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "handlerBuild.test")
}

// Without a toolchain there is nothing to ask, and guessing a command for a
// language the CLI cannot identify would run the wrong thing.
func (s *OSHandlerRunnerSuite) Test_no_toolchain_and_no_command_is_an_error() {
	runner := NewOSHandlerRunner(nil, s.logger)

	_, err := runner.Run(context.Background(), s.config(SuiteUnit))
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "--test-command")
}

func TestOSHandlerRunnerSuite(t *testing.T) { suite.Run(t, new(OSHandlerRunnerSuite)) }

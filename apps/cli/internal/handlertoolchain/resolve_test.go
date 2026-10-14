package handlertoolchain

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/consts"
	"github.com/stretchr/testify/suite"
)

type ResolveSuite struct {
	suite.Suite
	appDir string
}

func (s *ResolveSuite) SetupTest() { s.appDir = s.T().TempDir() }

func (s *ResolveSuite) writeGoMod(contents string) {
	s.Require().NoError(os.WriteFile(filepath.Join(s.appDir, "go.mod"), []byte(contents), 0o644))
}

func (s *ResolveSuite) validConfig() *Config {
	return &Config{Command: "cargo build --release", Binary: "target/release/handlers"}
}

// A runtime that hosts the language itself has no executable to build, so
// there is nothing to resolve and nothing to report.
func (s *ResolveSuite) Test_non_os_only_runtime_resolves_to_nothing() {
	for _, runtime := range []string{"nodejs24.x", "python3.13"} {
		toolchain, err := Resolve(Options{Runtime: runtime, AppDir: s.appDir})
		s.Require().NoError(err, runtime)
		s.Assert().Nil(toolchain, runtime)
	}
}

func (s *ResolveSuite) Test_go_module_requiring_the_sdk_resolves_to_the_go_toolchain() {
	s.writeGoMod("module example.com/app\n\ngo 1.26.0\n\nrequire " + GoSDKModule + " v0.1.0\n")

	toolchain, err := Resolve(Options{Runtime: consts.RuntimeOSOnlyDebian13, AppDir: s.appDir})
	s.Require().NoError(err)
	s.Require().NotNil(toolchain)
	s.Assert().Equal("go", toolchain.Name())
}

func (s *ResolveSuite) Test_handler_build_block_resolves_to_the_custom_toolchain() {
	toolchain, err := Resolve(Options{
		Runtime: consts.RuntimeOSOnlyDebian13,
		AppDir:  s.appDir,
		Config:  s.validConfig(),
	})
	s.Require().NoError(err)
	s.Require().NotNil(toolchain)
	s.Assert().Equal("custom", toolchain.Name())
}

// An author who wrote a handlerBuild block has said they want to drive their
// own build, so it wins over the Go project the directory also looks like.
func (s *ResolveSuite) Test_handler_build_block_beats_an_inferred_go_project() {
	s.writeGoMod("module example.com/app\n\nrequire " + GoSDKModule + " v0.1.0\n")

	toolchain, err := Resolve(Options{
		Runtime: consts.RuntimeOSOnlyDebian13,
		AppDir:  s.appDir,
		Config:  s.validConfig(),
	})
	s.Require().NoError(err)
	s.Assert().Equal("custom", toolchain.Name())
}

func (s *ResolveSuite) Test_os_only_with_no_signal_says_both_ways_to_fix_it() {
	_, err := Resolve(Options{Runtime: consts.RuntimeOSOnlyDebian13, AppDir: s.appDir})
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), GoSDKModule)
	s.Assert().Contains(err.Error(), "handlerBuild")
}

// A go.mod that does not require the SDK is a Go project that is not a
// Celerity application, which is not the same as a Go Celerity application.
func (s *ResolveSuite) Test_go_module_without_the_sdk_is_not_a_go_project() {
	s.writeGoMod("module example.com/app\n\ngo 1.26.0\n")

	_, err := Resolve(Options{Runtime: consts.RuntimeOSOnlyDebian13, AppDir: s.appDir})
	s.Require().Error(err)
}

func (s *ResolveSuite) Test_commented_out_requirement_is_not_a_requirement() {
	s.writeGoMod("module example.com/app\n\n// require " + GoSDKModule + " v0.1.0\n")

	_, err := Resolve(Options{Runtime: consts.RuntimeOSOnlyDebian13, AppDir: s.appDir})
	s.Require().Error(err)
}

// A module whose path merely starts with the SDK's is a different module.
func (s *ResolveSuite) Test_longer_module_path_is_not_the_sdk() {
	s.writeGoMod("module example.com/app\n\nrequire " + GoSDKModule + "-contrib v0.1.0\n")

	_, err := Resolve(Options{Runtime: consts.RuntimeOSOnlyDebian13, AppDir: s.appDir})
	s.Require().Error(err)
}

func (s *ResolveSuite) Test_invalid_handler_build_block_is_refused() {
	_, err := Resolve(Options{
		Runtime: consts.RuntimeOSOnlyDebian13,
		AppDir:  s.appDir,
		Config:  &Config{Binary: "target/release/handlers"},
	})
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "handlerBuild.command")
}

func TestResolveSuite(t *testing.T) { suite.Run(t, new(ResolveSuite)) }

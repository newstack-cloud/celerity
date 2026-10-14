package handlertoolchain

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CustomToolchainSuite struct {
	suite.Suite
	appDir string
	runner *stubRunner
}

func (s *CustomToolchainSuite) SetupTest() {
	s.appDir = s.T().TempDir()
	s.runner = &stubRunner{}
}

func (s *CustomToolchainSuite) toolchain(c *Config) *customToolchain {
	return newCustomToolchain(c, s.runner)
}

// writesBinaryAt makes the stub stand in for a build command that produced
// the executable the config names.
func (s *CustomToolchainSuite) writesBinaryAt(rel string) {
	s.runner.onRun = func(recordedCommand) error {
		path := filepath.Join(s.appDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("binary"), 0o644)
	}
}

func (s *CustomToolchainSuite) Test_BuildBinary_runs_the_command_and_collects_the_binary() {
	s.writesBinaryAt("target/release/handlers")
	tc := s.toolchain(&Config{Command: "cargo build --release", Binary: "target/release/handlers"})

	out := filepath.Join(s.T().TempDir(), "handlers")
	s.Require().NoError(tc.BuildBinary(context.Background(), BinaryParams{
		AppDir: s.appDir, OS: "linux", Arch: "arm64", OutputPath: out,
		DeployTarget: "aws-serverless", Mode: ModeDeploy,
	}))

	s.Require().Len(s.runner.commands, 1)
	s.Assert().Equal("sh -c cargo build --release", s.runner.lines()[0])
	s.Assert().FileExists(out)

	info, err := os.Stat(out)
	s.Require().NoError(err)
	s.Assert().NotZero(info.Mode()&0o111, "expected the collected binary to be executable")
}

// A build script keys off these rather than hard-coding one platform.
func (s *CustomToolchainSuite) Test_BuildBinary_injects_the_build_environment() {
	s.writesBinaryAt("out/handlers")
	tc := s.toolchain(&Config{
		Command: "make", Binary: "out/handlers",
		Env: map[string]string{"RUSTFLAGS": "-C target-feature=+crt-static"},
	})

	out := filepath.Join(s.T().TempDir(), "handlers")
	s.Require().NoError(tc.BuildBinary(context.Background(), BinaryParams{
		AppDir: s.appDir, OS: "linux", Arch: "amd64", OutputPath: out,
		DeployTarget: "aws-serverless", Mode: ModeDev,
	}))

	env := s.runner.commands[0].Env
	s.Assert().Equal("linux", env[EnvBuildOS])
	s.Assert().Equal("amd64", env[EnvBuildArch])
	s.Assert().Equal(out, env[EnvBuildOutput])
	s.Assert().Equal("aws-serverless", env[EnvBuildDeployTarget])
	s.Assert().Equal(ModeDev, env[EnvBuildMode])
	s.Assert().Equal("-C target-feature=+crt-static", env["RUSTFLAGS"])
}

// A cross-compiled binary lands somewhere different per target.
func (s *CustomToolchainSuite) Test_BuildBinary_applies_the_per_target_override() {
	s.writesBinaryAt("target/aarch64/release/handlers")
	tc := s.toolchain(&Config{
		Command: "cargo build --release",
		Binary:  "target/release/handlers",
		Targets: map[string]*Config{
			"aws-serverless": {Binary: "target/aarch64/release/handlers"},
		},
	})

	out := filepath.Join(s.T().TempDir(), "handlers")
	s.Require().NoError(tc.BuildBinary(context.Background(), BinaryParams{
		AppDir: s.appDir, OS: "linux", Arch: "arm64", OutputPath: out,
		DeployTarget: "aws-serverless",
	}))
	s.Assert().FileExists(out)
	// The override named only the binary, so the base command still stands.
	s.Assert().Equal("sh -c cargo build --release", s.runner.lines()[0])
}

func (s *CustomToolchainSuite) Test_BuildBinary_says_which_setting_is_wrong_when_the_binary_is_absent() {
	tc := s.toolchain(&Config{Command: "true", Binary: "target/release/handlers"})

	err := tc.BuildBinary(context.Background(), BinaryParams{
		AppDir: s.appDir, OutputPath: filepath.Join(s.T().TempDir(), "handlers"),
	})
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "handlerBuild.binary")
}

// A command that writes straight to CELERITY_BUILD_OUTPUT has already put the
// executable where it belongs.
func (s *CustomToolchainSuite) Test_BuildBinary_accepts_a_command_that_writes_to_the_output_path() {
	out := filepath.Join(s.appDir, "build", "handlers")
	s.runner.onRun = func(c recordedCommand) error {
		path := c.Env[EnvBuildOutput]
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("binary"), 0o644)
	}
	tc := s.toolchain(&Config{Command: "build.sh", Binary: "build/handlers"})

	s.Require().NoError(tc.BuildBinary(context.Background(), BinaryParams{
		AppDir: s.appDir, OutputPath: out,
	}))
	info, err := os.Stat(out)
	s.Require().NoError(err)
	s.Assert().NotZero(info.Mode() & 0o111)
}

// Without an extractor the blueprint's hand-authored handlers stand alone,
// which is not an error.
func (s *CustomToolchainSuite) Test_Extract_without_an_extractor_reports_nothing() {
	tc := s.toolchain(&Config{Command: "make", Binary: "out/handlers"})

	manifest, err := tc.Extract(context.Background(), ExtractParams{AppDir: s.appDir})
	s.Require().NoError(err)
	s.Assert().Nil(manifest)
	s.Assert().Empty(s.runner.commands)
}

func (s *CustomToolchainSuite) Test_Extract_reads_the_manifest_from_stdout() {
	s.runner.output = []byte(`{"version":"1","functionHandlers":[{"resourceName":"getOrder"}]}`)
	tc := s.toolchain(&Config{
		Command: "make", Binary: "out/handlers",
		Extract: &ExtractConfig{Command: "./scripts/extract"},
	})

	manifest, err := tc.Extract(context.Background(), ExtractParams{AppDir: s.appDir})
	s.Require().NoError(err)
	s.Require().NotNil(manifest)
	s.Assert().Len(manifest.FunctionHandlers, 1)
}

func (s *CustomToolchainSuite) Test_Extract_reads_the_manifest_from_a_declared_file() {
	s.runner.onRun = func(recordedCommand) error {
		return os.WriteFile(
			filepath.Join(s.appDir, "manifest.json"),
			[]byte(`{"version":"1","functionHandlers":[{"resourceName":"getOrder"}]}`), 0o644)
	}
	tc := s.toolchain(&Config{
		Command: "make", Binary: "out/handlers",
		Extract: &ExtractConfig{Command: "./scripts/extract", ManifestFile: "manifest.json"},
	})

	manifest, err := tc.Extract(context.Background(), ExtractParams{AppDir: s.appDir})
	s.Require().NoError(err)
	s.Require().NotNil(manifest)
	s.Assert().Len(manifest.FunctionHandlers, 1)
}

// The merge skips what it does not recognise, so an unusable manifest would
// otherwise produce an application with no handlers and no complaint.
func (s *CustomToolchainSuite) Test_Extract_refuses_a_manifest_the_merge_would_silently_skip() {
	s.runner.output = []byte(`{"version":"1","functionHandlers":[{"exportName":"getOrder"}]}`)
	tc := s.toolchain(&Config{
		Command: "make", Binary: "out/handlers",
		Extract: &ExtractConfig{Command: "./scripts/extract"},
	})

	_, err := tc.Extract(context.Background(), ExtractParams{AppDir: s.appDir})
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "resourceName")
}

func (s *CustomToolchainSuite) Test_TestCommand_reports_only_configured_suites() {
	tc := s.toolchain(&Config{
		Command: "make", Binary: "out/handlers",
		Test: map[string]string{"unit": "cargo test"},
	})

	cmd, ok := tc.TestCommand("unit")
	s.Require().True(ok)
	s.Assert().Equal([]string{"sh", "-c", "cargo test"}, cmd)

	_, ok = tc.TestCommand("integration")
	s.Assert().False(ok)
}

// Watching the CLI's own build output would make each rebuild trigger the next.
func (s *CustomToolchainSuite) Test_WatchPaths_always_skips_the_celerity_directory() {
	tc := s.toolchain(&Config{
		Command: "make", Binary: "out/handlers",
		WatchExtensions: []string{".rs"},
		WatchSkipDirs:   []string{"target"},
	})

	paths := tc.WatchPaths()
	s.Assert().Equal([]string{"."}, paths.Roots)
	s.Assert().Equal([]string{".rs"}, paths.Extensions)
	s.Assert().Contains(paths.SkipDirs, "target")
	s.Assert().Contains(paths.SkipDirs, ".celerity")
}

func (s *CustomToolchainSuite) Test_forTarget_merges_env_rather_than_replacing_it() {
	cfg := &Config{
		Command: "make", Binary: "out/handlers",
		Env: map[string]string{"A": "1", "B": "2"},
		Targets: map[string]*Config{
			"aws-serverless": {Env: map[string]string{"B": "override", "C": "3"}},
		},
	}

	merged := cfg.forTarget("aws-serverless")
	s.Assert().Equal(map[string]string{"A": "1", "B": "override", "C": "3"}, merged.Env)
	s.Assert().Equal("make", merged.Command)
}

func (s *CustomToolchainSuite) Test_Validate_rejects_nested_target_overrides() {
	cfg := &Config{
		Command: "make", Binary: "out/handlers",
		Targets: map[string]*Config{
			"aws-serverless": {Targets: map[string]*Config{"aws": {}}},
		},
	}
	s.Assert().Error(cfg.Validate())
}

func TestCustomToolchainSuite(t *testing.T) { suite.Run(t, new(CustomToolchainSuite)) }

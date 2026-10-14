package handlertoolchain

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
	"github.com/stretchr/testify/suite"
)

type GoToolchainSuite struct {
	suite.Suite
	appDir string
	runner *stubRunner
	tc     *goToolchain
}

func (s *GoToolchainSuite) SetupTest() {
	s.appDir = s.T().TempDir()
	s.runner = &stubRunner{}
	s.tc = newGoToolchain(s.runner)
	// Resolved from the env var so the suite does not need celerity-go
	// installed on the host.
	s.T().Setenv(GoToolEnvVar, "/usr/local/bin/celerity-go")
}

func (s *GoToolchainSuite) prepare(p PrepareParams) error {
	p.AppDir = s.appDir
	return s.tc.Prepare(context.Background(), p)
}

func (s *GoToolchainSuite) Test_Prepare_generates_for_the_deploy_target_then_reconciles_the_module() {
	s.Require().NoError(s.prepare(PrepareParams{DeployTarget: "aws-serverless", Mode: ModeDeploy}))

	s.Require().Len(s.runner.commands, 2)
	s.Assert().Contains(s.runner.lines()[0], "generate --target aws-serverless --package .")
	s.Assert().Equal("go mod tidy", s.runner.lines()[1])
}

// A dev session reads its own config store and talks to local emulators, so
// it links packages a deployed artefact must not carry.
func (s *GoToolchainSuite) Test_Prepare_passes_local_only_in_dev_mode() {
	s.Require().NoError(s.prepare(PrepareParams{DeployTarget: "aws-serverless", Mode: ModeDev}))
	s.Assert().Contains(s.runner.lines()[0], "--local")

	s.runner.commands = nil
	s.Require().NoError(s.prepare(PrepareParams{DeployTarget: "aws-serverless", Mode: ModeDeploy}))
	s.Assert().NotContains(s.runner.lines()[0], "--local")
}

func (s *GoToolchainSuite) Test_Prepare_passes_blueprint_resources_and_engines() {
	s.Require().NoError(s.prepare(PrepareParams{
		DeployTarget:  "aws-serverless",
		Mode:          ModeDeploy,
		ResourceKinds: []string{"bucket", "datastore"},
		SQLEngines:    []string{"postgres"},
		Telemetry:     "otel",
	}))

	line := s.runner.lines()[0]
	s.Assert().Contains(line, "--resource bucket,datastore")
	s.Assert().Contains(line, "--sql-engine postgres")
	s.Assert().Contains(line, "--telemetry otel")
}

// A test that reaches a real resource has to reach it through the same
// provider the application does, and in Go that means the provider package is
// linked into the test binary too.
func (s *GoToolchainSuite) Test_Prepare_generates_for_the_integration_suites_when_asked() {
	s.Require().NoError(s.prepare(PrepareParams{
		DeployTarget: "aws-serverless",
		Mode:         ModeDev,
		TestPackages: true,
	}))

	s.Assert().Contains(s.runner.lines()[0], "--test-packages")
}

// Nothing in a deployed artefact reads them, so generating them would write
// files into source for no reader.
func (s *GoToolchainSuite) Test_Prepare_leaves_the_test_suites_alone_by_default() {
	s.Require().NoError(s.prepare(PrepareParams{DeployTarget: "aws-serverless", Mode: ModeDeploy}))

	s.Assert().NotContains(s.runner.lines()[0], "--test-packages")
}

func (s *GoToolchainSuite) Test_Prepare_explains_why_the_module_had_to_be_reconciled() {
	s.runner.failOn = "go mod tidy"

	err := s.prepare(PrepareParams{DeployTarget: "aws-serverless", Mode: ModeDeploy})
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "platform imports")
}

// Statically linked, because the base OS differs between the image the
// executable is built against and the ones it is deployed to.
func (s *GoToolchainSuite) Test_BuildBinary_builds_a_static_stripped_binary_for_the_target() {
	out := filepath.Join(s.appDir, ".celerity", "dev", "handlers")
	s.runner.onRun = func(recordedCommand) error {
		return os.WriteFile(out, []byte("binary"), 0o644)
	}

	err := s.tc.BuildBinary(context.Background(), BinaryParams{
		AppDir: s.appDir, OS: "linux", Arch: "arm64", OutputPath: out,
	})
	s.Require().NoError(err)

	s.Require().Len(s.runner.commands, 1)
	cmd := s.runner.commands[0]
	s.Assert().Equal("go", cmd.Name)
	s.Assert().Contains(cmd.line(), "-trimpath")
	s.Assert().Equal("0", cmd.Env["CGO_ENABLED"])
	s.Assert().Equal("linux", cmd.Env["GOOS"])
	s.Assert().Equal("arm64", cmd.Env["GOARCH"])
}

// The runtime starts the executable as a child process, so a build that left
// it unreadable as a program fails as a bare permission error.
func (s *GoToolchainSuite) Test_BuildBinary_leaves_the_executable_bit_set() {
	out := filepath.Join(s.appDir, "handlers")
	s.runner.onRun = func(recordedCommand) error {
		return os.WriteFile(out, []byte("binary"), 0o600)
	}

	s.Require().NoError(s.tc.BuildBinary(context.Background(), BinaryParams{
		AppDir: s.appDir, OS: "linux", Arch: "arm64", OutputPath: out,
	}))

	info, err := os.Stat(out)
	s.Require().NoError(err)
	s.Assert().NotZero(info.Mode()&0o111, "expected the executable bit to be set")
}

func (s *GoToolchainSuite) Test_Extract_reads_the_manifest_from_the_file_the_tool_writes() {
	s.runner.onRun = func(c recordedCommand) error {
		// celerity-go prints a human summary and writes the manifest to the
		// path given by --out, so there is nothing to read off stdout.
		return os.WriteFile(outFlag(c.Args), []byte(`{
			"version": "v1",
			"handlers": [],
			"functionHandlers": [{"resourceName": "getOrder", "exportName": "getOrder"}],
			"guardHandlers": []
		}`), 0o644)
	}

	manifest, err := s.tc.Extract(context.Background(), ExtractParams{AppDir: s.appDir})
	s.Require().NoError(err)
	s.Require().NotNil(manifest)
	s.Assert().Len(manifest.FunctionHandlers, 1)
	s.Assert().Equal("getOrder", manifest.FunctionHandlers[0].ResourceName)
}

func (s *GoToolchainSuite) Test_Extract_rejects_an_unusable_manifest() {
	s.runner.onRun = func(c recordedCommand) error {
		return os.WriteFile(outFlag(c.Args), []byte(`{"version":"v1","functionHandlers":[{"exportName":"x"}]}`), 0o644)
	}

	_, err := s.tc.Extract(context.Background(), ExtractParams{AppDir: s.appDir})
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "resourceName")
}

func (s *GoToolchainSuite) Test_Validate_requires_the_sdk_in_go_mod() {
	err := s.tc.Validate(s.appDir)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "go get "+GoSDKModule)

	s.Require().NoError(os.WriteFile(
		filepath.Join(s.appDir, "go.mod"),
		[]byte("module example.com/app\n\nrequire "+GoSDKModule+" v0.1.0\n"), 0o644))
	s.Assert().NoError(s.tc.Validate(s.appDir))
}

func (s *GoToolchainSuite) Test_the_tool_is_taken_from_the_env_override_first() {
	s.T().Setenv(GoToolEnvVar, "/opt/celerity/celerity-go")

	prefix, err := goToolCommand(s.appDir)

	s.Require().NoError(err)
	s.Assert().Equal([]string{"/opt/celerity/celerity-go"}, prefix)
}

// Preferred over PATH, because a tool dependency is resolved by the same
// version selection as the SDK requirement beside it while one on PATH is
// whatever was installed last.
func (s *GoToolchainSuite) Test_a_tool_dependency_is_preferred_over_one_on_the_path() {
	s.T().Setenv(GoToolEnvVar, "")
	s.onPath(GoToolBinary)
	s.writeGoMod("module example.com/app\n\nrequire " + GoSDKModule + " v0.2.1\n" +
		"\ntool " + GoToolModule + "\n")

	prefix, err := goToolCommand(s.appDir)

	s.Require().NoError(err)
	s.Assert().Equal([]string{"go", "tool", GoToolBinary}, prefix)
}

func (s *GoToolchainSuite) Test_a_tool_dependency_inside_a_block_counts_too() {
	s.T().Setenv(GoToolEnvVar, "")
	s.T().Setenv("PATH", s.T().TempDir())
	s.writeGoMod("module example.com/app\n\ntool (\n\t" + GoToolModule + "\n)\n")

	prefix, err := goToolCommand(s.appDir)

	s.Require().NoError(err)
	s.Assert().Equal([]string{"go", "tool", GoToolBinary}, prefix)
}

func (s *GoToolchainSuite) Test_the_path_is_used_where_nothing_declares_the_tool() {
	s.T().Setenv(GoToolEnvVar, "")
	binary := s.onPath(GoToolBinary)
	s.writeGoMod("module example.com/app\n\nrequire " + GoSDKModule + " v0.2.1\n")

	prefix, err := goToolCommand(s.appDir)

	s.Require().NoError(err)
	s.Assert().Equal([]string{binary}, prefix)
}

// At the version the application already requires rather than at whatever is
// newest, since generate writes imports of that version's packages.
func (s *GoToolchainSuite) Test_an_absent_tool_names_the_command_that_adds_it() {
	s.T().Setenv(GoToolEnvVar, "")
	s.T().Setenv("PATH", s.T().TempDir())
	s.writeGoMod("module example.com/app\n\nrequire " + GoSDKModule + " v0.2.1\n")

	_, err := goToolCommand(s.appDir)

	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "go get -tool "+GoToolModule+"@v0.2.1")
	s.Assert().Contains(err.Error(), GoToolEnvVar)
}

func (s *GoToolchainSuite) Test_an_absent_tool_omits_a_version_it_cannot_read() {
	s.T().Setenv(GoToolEnvVar, "")
	s.T().Setenv("PATH", s.T().TempDir())

	_, err := goToolCommand(s.appDir)

	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "go get -tool "+GoToolModule+"\n")
}

// A path that merely begins with the SDK's is a different module.
func (s *GoToolchainSuite) Test_a_lookalike_module_is_not_the_tool() {
	s.T().Setenv(GoToolEnvVar, "")
	s.T().Setenv("PATH", s.T().TempDir())
	s.writeGoMod("module example.com/app\n\ntool " + GoToolModule + "-contrib\n")

	_, err := goToolCommand(s.appDir)

	s.Require().Error(err)
}

func (s *GoToolchainSuite) writeGoMod(contents string) {
	s.T().Helper()
	s.Require().NoError(os.WriteFile(filepath.Join(s.appDir, "go.mod"), []byte(contents), 0o755))
}

// onPath puts an executable of that name on PATH and answers with its path.
func (s *GoToolchainSuite) onPath(name string) string {
	s.T().Helper()
	dir := s.T().TempDir()
	path := filepath.Join(dir, name)
	s.Require().NoError(os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755))
	s.T().Setenv("PATH", dir)
	return path
}

func (s *GoToolchainSuite) Test_WatchPaths_covers_the_module_root_and_skips_build_output() {
	paths := s.tc.WatchPaths()
	s.Assert().Equal([]string{"."}, paths.Roots)
	s.Assert().Equal([]string{".go"}, paths.Extensions)
	s.Assert().Contains(paths.SkipDirs, ".celerity")
	s.Assert().Contains(paths.SkipFiles, goGeneratedFile,
		"Prepare rewrites it, so reacting to it would make each rebuild trigger the next")
}

// outFlag reports the value of --out in a recorded command.
func outFlag(args []string) string {
	for i, arg := range args {
		if arg == "--out" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestGoToolchainSuite(t *testing.T) { suite.Run(t, new(GoToolchainSuite)) }

// The manifest the Go SDK emits declares "v1" while the CLI's own fallback
// constructs "1". Both have been in the wild unvalidated, so both are read.
func TestValidateManifest_acceptsBothVersionSpellings(t *testing.T) {
	for _, version := range []string{"1", "v1"} {
		var m preprocess.HandlerManifest
		err := json.Unmarshal([]byte(`{"version":"`+version+`"}`), &m)
		if err != nil {
			t.Fatal(err)
		}
		if err := preprocess.ValidateManifest(&m); err != nil {
			t.Fatalf("version %q: %v", version, err)
		}
	}
}

func TestValidateManifest_namesTheOffendingAnnotation(t *testing.T) {
	m := &preprocess.HandlerManifest{
		Version: "v1",
		FunctionHandlers: []preprocess.FunctionHandler{{
			ResourceName: "getOrder",
			Annotations:  map[string]any{"handler.http": true},
		}},
	}
	err := preprocess.ValidateManifest(m)
	if err == nil || !strings.Contains(err.Error(), "handler.http") {
		t.Fatalf("expected the bad annotation key to be named, got %v", err)
	}
}

package devconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/celerity/apps/cli/internal/consts"
	"github.com/newstack-cloud/celerity/apps/cli/internal/docker"
	"github.com/stretchr/testify/suite"
)

type DevToolchainSuite struct {
	suite.Suite
	appDir string
}

func (s *DevToolchainSuite) SetupTest() { s.appDir = s.T().TempDir() }

func (s *DevToolchainSuite) writeDeployConfig(contents string) string {
	path := filepath.Join(s.appDir, "app.deploy.jsonc")
	s.Require().NoError(os.WriteFile(path, []byte(contents), 0o644))
	return path
}

func (s *DevToolchainSuite) Test_ReadHandlerBuild_absent_block_is_not_an_error() {
	path := s.writeDeployConfig(`{"appName":"app","deployTarget":{"name":"aws"}}`)

	cfg, err := ReadHandlerBuild(path)
	s.Require().NoError(err)
	s.Assert().Nil(cfg)
}

func (s *DevToolchainSuite) Test_ReadHandlerBuild_reads_the_block_through_comments() {
	path := s.writeDeployConfig(`{
		// The deploy target this app goes to.
		"appName": "app",
		"deployTarget": { "name": "aws" },
		"handlerBuild": {
			"command": "cargo build --release",
			"binary": "target/release/handlers",
			"watchExtensions": [".rs"],
			"test": { "unit": "cargo test" }
		}
	}`)

	cfg, err := ReadHandlerBuild(path)
	s.Require().NoError(err)
	s.Require().NotNil(cfg)
	s.Assert().Equal("cargo build --release", cfg.Command)
	s.Assert().Equal("target/release/handlers", cfg.Binary)
	s.Assert().Equal([]string{".rs"}, cfg.WatchExtensions)
	s.Assert().Equal("cargo test", cfg.Test["unit"])
}

func (s *DevToolchainSuite) Test_ReadHandlerBuild_reports_an_incomplete_block() {
	path := s.writeDeployConfig(`{"deployTarget":{"name":"aws"},"handlerBuild":{"command":"make"}}`)

	_, err := ReadHandlerBuild(path)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "handlerBuild.binary")
}

func (s *DevToolchainSuite) Test_resolveToolchain_is_nothing_for_a_hosted_language() {
	tc, err := resolveToolchain("nodejs24.x", s.appDir, "")
	s.Require().NoError(err)
	s.Assert().Nil(tc)
}

func (s *DevToolchainSuite) Test_resolveToolchain_uses_the_handler_build_block() {
	path := s.writeDeployConfig(`{
		"deployTarget": {"name":"aws"},
		"handlerBuild": {"command":"make","binary":"out/handlers"}
	}`)

	tc, err := resolveToolchain(consts.RuntimeOSOnlyDebian13, s.appDir, path)
	s.Require().NoError(err)
	s.Require().NotNil(tc)
	s.Assert().Equal("custom", tc.Name())
}

func (s *DevToolchainSuite) Test_resolveToolchain_infers_a_go_project() {
	s.Require().NoError(os.WriteFile(
		filepath.Join(s.appDir, "go.mod"),
		[]byte("module example.com/app\n\nrequire github.com/newstack-cloud/celerity-go-sdk v0.1.0\n"),
		0o644))

	tc, err := resolveToolchain(consts.RuntimeOSOnlyDebian13, s.appDir, "")
	s.Require().NoError(err)
	s.Require().NotNil(tc)
	s.Assert().Equal("go", tc.Name())
}

// The application directory is already mounted, so the executable needs no
// mount of its own.
func (s *DevToolchainSuite) Test_the_container_path_sits_under_the_mounted_app_dir() {
	s.Assert().Equal("/opt/celerity/app/.celerity/dev/handlers", ContainerHandlersExecutable())
}

type OSOnlyContainerSuite struct {
	suite.Suite
}

// emptyBlueprint is a blueprint with no resources, which is enough for the
// container config: what it reads from one is per-resource config store ids.
func emptyBlueprint() *schema.Blueprint {
	return &schema.Blueprint{Resources: &schema.ResourceMap{Values: map[string]*schema.Resource{}}}
}

func (s *OSOnlyContainerSuite) containerConfig(osOnly bool, modulePath string) *docker.ContainerConfig {
	return buildContainerConfig(
		"ghcr.io/newstack-cloud/celerity-runtime-core:dev-0.3.0",
		"orders", "8080", "/app", "/opt/celerity/merged.blueprint.yaml",
		modulePath, nil, emptyBlueprint(), "aws", false, osOnly,
	)
}

func (s *OSOnlyContainerSuite) config(osOnly bool) map[string]string {
	return s.containerConfig(osOnly, "main.go").EnvVars
}

// The core runtime has to be told where the executable it starts lives: the
// image default is where a derivative image puts one, not where a dev session
// builds one.
func (s *OSOnlyContainerSuite) Test_os_only_is_told_where_the_executable_is() {
	env := s.config(true)
	s.Assert().Equal(ContainerHandlersExecutable(), env["CELERITY_HANDLERS_EXECUTABLE"])
}

// There is no module to load; the executable is the module.
func (s *OSOnlyContainerSuite) Test_os_only_has_no_module_path() {
	s.Assert().NotContains(s.config(true), "CELERITY_MODULE_PATH")
	s.Assert().Contains(s.config(false), "CELERITY_MODULE_PATH")
}

// The core runtime reads no arguments, so "dev" is an argument nothing
// consumes.
func (s *OSOnlyContainerSuite) Test_os_only_is_started_with_no_arguments() {
	s.Assert().Nil(s.containerConfig(true, "main.go").Cmd)
	s.Assert().Equal([]string{"dev"}, s.containerConfig(false, "src/app.module.ts").Cmd)
}

// The blueprint is mounted outside the app directory, so the image default
// does not point at it.
func (s *OSOnlyContainerSuite) Test_the_blueprint_path_is_where_the_cli_mounts_it() {
	s.Assert().Equal("/opt/celerity/merged.blueprint.yaml", s.config(true)["CELERITY_BLUEPRINT"])
}

func TestDevToolchainSuite(t *testing.T)    { suite.Run(t, new(DevToolchainSuite)) }
func TestOSOnlyContainerSuite(t *testing.T) { suite.Run(t, new(OSOnlyContainerSuite)) }

package deployconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
	"github.com/stretchr/testify/suite"
)

type HandlerBuildConfigSuite struct {
	suite.Suite
	dir string
}

func (s *HandlerBuildConfigSuite) SetupTest() { s.dir = s.T().TempDir() }

func (s *HandlerBuildConfigSuite) writeConfig(contents string) string {
	path := filepath.Join(s.dir, "app.deploy.jsonc")
	s.Require().NoError(os.WriteFile(path, []byte(contents), 0o644))
	return path
}

func (s *HandlerBuildConfigSuite) Test_handler_build_is_read_off_the_deploy_config() {
	path := s.writeConfig(`{
		"appName": "orders",
		"deployTarget": { "name": "aws-serverless" },
		"handlerBuild": {
			"command": "cargo build --release",
			"binary": "target/release/handlers",
			"targets": {
				"aws-serverless": { "binary": "target/aarch64-unknown-linux-musl/release/handlers" }
			}
		}
	}`)

	cfg, err := ReadAppDeployConfig(path)
	s.Require().NoError(err)
	s.Require().NotNil(cfg.HandlerBuild)
	s.Assert().Equal("cargo build --release", cfg.HandlerBuild.Command)
	s.Assert().Contains(cfg.HandlerBuild.Targets, "aws-serverless")
}

func (s *HandlerBuildConfigSuite) Test_a_config_without_the_block_leaves_it_nil() {
	path := s.writeConfig(`{"appName":"orders","deployTarget":{"name":"aws-serverless"}}`)

	cfg, err := ReadAppDeployConfig(path)
	s.Require().NoError(err)
	s.Assert().Nil(cfg.HandlerBuild)
}

// handlerBuild describes a local build step. The deploy engine has no use for
// it, and a build command in engine-bound config would be sent to a remote
// engine on every deploy.
func (s *HandlerBuildConfigSuite) Test_handler_build_is_not_forwarded_to_the_deploy_engine() {
	cfg := &AppDeployConfig{
		AppName: "orders",
		DeployTarget: DeployTarget{
			Name:   "aws-serverless",
			AppEnv: "production",
		},
		HandlerBuild: &handlertoolchain.Config{
			Command: "cargo build --release",
			Binary:  "target/release/handlers",
			Env:     map[string]string{"SECRET_TOKEN": "not-for-the-engine"},
		},
	}

	converted, err := ConvertToBluelinkFormat(cfg)
	s.Require().NoError(err)

	encoded, err := json.Marshal(converted)
	s.Require().NoError(err)
	s.Assert().NotContains(string(encoded), "handlerBuild")
	s.Assert().NotContains(string(encoded), "cargo build")
	s.Assert().NotContains(string(encoded), "not-for-the-engine")
}

func TestHandlerBuildConfigSuite(t *testing.T) { suite.Run(t, new(HandlerBuildConfigSuite)) }

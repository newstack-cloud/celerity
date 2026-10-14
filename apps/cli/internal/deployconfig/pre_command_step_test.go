package deployconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/build"
	"github.com/newstack-cloud/deploy-cli-sdk/config"
	"github.com/stretchr/testify/suite"
)

type PreCommandStepSuite struct {
	suite.Suite
	origDir string
}

func TestPreCommandStepSuite(t *testing.T) {
	suite.Run(t, new(PreCommandStepSuite))
}

func (s *PreCommandStepSuite) SetupTest() {
	dir := s.T().TempDir()
	origDir, err := os.Getwd()
	s.Require().NoError(err)
	s.origDir = origDir
	s.Require().NoError(os.Chdir(dir))
}

func (s *PreCommandStepSuite) TearDownTest() {
	s.Require().NoError(os.Chdir(s.origDir))
}

func (s *PreCommandStepSuite) newProvider(protocol, endpoint string) *config.Provider {
	p := config.NewProvider()
	p.BindEnvVar("connectProtocol", "CELERITY_CLI_TEST_CONNECT_PROTOCOL")
	p.BindEnvVar("engineEndpoint", "CELERITY_CLI_TEST_ENGINE_ENDPOINT")
	s.T().Setenv("CELERITY_CLI_TEST_CONNECT_PROTOCOL", protocol)
	s.T().Setenv("CELERITY_CLI_TEST_ENGINE_ENDPOINT", endpoint)
	return p
}

func (s *PreCommandStepSuite) writeManifest(manifest *build.Manifest) {
	s.Require().NoError(build.WriteManifest(manifest))
}

func (s *PreCommandStepSuite) Test_injectBuildManifestPointer_localEngine_usesAbsoluteLocalPath() {
	s.writeManifest(&build.Manifest{
		Version:  build.ManifestVersion,
		Runtime:  "nodejs24.x",
		Target:   "aws-serverless",
		Handlers: map[string]*build.HandlerArtifacts{},
	})

	appCfg := &AppDeployConfig{}
	provider := s.newProvider("unix", "")

	err := injectBuildManifestPointer(appCfg, provider)
	s.Require().NoError(err)

	absPath, _ := filepath.Abs(build.ManifestFile)
	s.Equal(absPath, appCfg.ContextVariables["celerity.buildManifest"])
}

func (s *PreCommandStepSuite) Test_injectBuildManifestPointer_remoteEngine_usesRemoteURL() {
	s.writeManifest(&build.Manifest{
		Version:   build.ManifestVersion,
		Runtime:   "nodejs24.x",
		Target:    "aws-serverless",
		Handlers:  map[string]*build.HandlerArtifacts{},
		RemoteURL: "s3://celerity-build-123-us-east-1/default/build-manifest.json",
	})

	appCfg := &AppDeployConfig{}
	provider := s.newProvider("tcp", "http://engine.example.com:8325")

	err := injectBuildManifestPointer(appCfg, provider)
	s.Require().NoError(err)

	s.Equal(
		"s3://celerity-build-123-us-east-1/default/build-manifest.json",
		appCfg.ContextVariables["celerity.buildManifest"],
	)
}

func (s *PreCommandStepSuite) Test_injectBuildManifestPointer_remoteEngine_missingRemoteURL_returnsError() {
	s.writeManifest(&build.Manifest{
		Version:  build.ManifestVersion,
		Runtime:  "nodejs24.x",
		Target:   "aws-serverless",
		Handlers: map[string]*build.HandlerArtifacts{},
	})

	appCfg := &AppDeployConfig{}
	provider := s.newProvider("tcp", "http://engine.example.com:8325")

	err := injectBuildManifestPointer(appCfg, provider)
	s.Require().Error(err)
	s.Contains(err.Error(), "remote deploy engine requires an uploaded build manifest")
	s.Nil(appCfg.ContextVariables["celerity.buildManifest"])
}

func (s *PreCommandStepSuite) Test_injectBuildManifestPointer_tcpLocalhost_treatedAsLocal() {
	s.writeManifest(&build.Manifest{
		Version:  build.ManifestVersion,
		Runtime:  "nodejs24.x",
		Target:   "aws-serverless",
		Handlers: map[string]*build.HandlerArtifacts{},
	})

	appCfg := &AppDeployConfig{}
	provider := s.newProvider("tcp", "http://localhost:8325")

	err := injectBuildManifestPointer(appCfg, provider)
	s.Require().NoError(err)

	absPath, _ := filepath.Abs(build.ManifestFile)
	s.Equal(absPath, appCfg.ContextVariables["celerity.buildManifest"])
}

func (s *PreCommandStepSuite) Test_injectBuildManifestPointer_noManifest_noop() {
	appCfg := &AppDeployConfig{}
	provider := s.newProvider("tcp", "http://engine.example.com:8325")

	err := injectBuildManifestPointer(appCfg, provider)
	s.Require().NoError(err)
	s.Nil(appCfg.ContextVariables)
}

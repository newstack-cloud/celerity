package handlerresolve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SDKDetectTestSuite struct {
	suite.Suite
}

func TestSDKDetectTestSuite(t *testing.T) {
	suite.Run(t, new(SDKDetectTestSuite))
}

// --- Node.js ---

func (s *SDKDetectTestSuite) Test_node_celerity_core_in_dependencies_passes() {
	appDir := s.newProject(`{"name":"x","dependencies":{"@celerity-sdk/core":"^0.6.0"}}`, "")
	s.NoError(RequireCelerityRuntimeSDK(appDir, "nodejs24.x"))
}

func (s *SDKDetectTestSuite) Test_node_celerity_scoped_package_in_dev_dependencies_passes() {
	appDir := s.newProject(`{"name":"x","devDependencies":{"@celerity-sdk/testing":"^0.6.0"}}`, "")
	s.NoError(RequireCelerityRuntimeSDK(appDir, "nodejs22.x"))
}

func (s *SDKDetectTestSuite) Test_node_celerity_in_peer_dependencies_passes() {
	appDir := s.newProject(`{"name":"x","peerDependencies":{"@celerity-sdk/core":"^0.6.0"}}`, "")
	s.NoError(RequireCelerityRuntimeSDK(appDir, "nodejs24.x"))
}

func (s *SDKDetectTestSuite) Test_node_no_celerity_package_fails() {
	appDir := s.newProject(`{"name":"x","dependencies":{"express":"^4.0.0"}}`, "")
	err := RequireCelerityRuntimeSDK(appDir, "nodejs24.x")
	s.Require().Error(err)
	s.Contains(err.Error(), "@celerity-sdk")
}

func (s *SDKDetectTestSuite) Test_node_missing_package_json_fails() {
	appDir := s.T().TempDir()
	err := RequireCelerityRuntimeSDK(appDir, "nodejs24.x")
	s.Require().Error(err)
	s.Contains(err.Error(), "@celerity-sdk")
}

func (s *SDKDetectTestSuite) Test_node_malformed_package_json_fails() {
	appDir := s.newProject(`{not json`, "")
	err := RequireCelerityRuntimeSDK(appDir, "nodejs24.x")
	s.Require().Error(err)
}

// --- Python ---

func (s *SDKDetectTestSuite) Test_python_celerity_sdk_in_pep621_dependencies_passes() {
	appDir := s.newProject("", `
[project]
name = "myapp"
dependencies = [
  "celerity-sdk>=0.6.0",
  "boto3",
]
`)
	s.NoError(RequireCelerityRuntimeSDK(appDir, "python3.13"))
}

func (s *SDKDetectTestSuite) Test_python_celerity_sdk_in_poetry_dependencies_passes() {
	appDir := s.newProject("", `
[tool.poetry.dependencies]
python = "^3.13"
celerity-sdk = "^0.6.0"
`)
	s.NoError(RequireCelerityRuntimeSDK(appDir, "python3.13"))
}

func (s *SDKDetectTestSuite) Test_python_no_celerity_package_fails() {
	appDir := s.newProject("", `
[project]
name = "myapp"
dependencies = ["boto3"]
`)
	err := RequireCelerityRuntimeSDK(appDir, "python3.13")
	s.Require().Error(err)
	s.Contains(err.Error(), "celerity-sdk")
}

func (s *SDKDetectTestSuite) Test_python_missing_pyproject_fails() {
	appDir := s.T().TempDir()
	err := RequireCelerityRuntimeSDK(appDir, "python3.13")
	s.Require().Error(err)
	s.Contains(err.Error(), "celerity-sdk")
}

func (s *SDKDetectTestSuite) Test_python_celerity_sdk_boundary_avoids_false_positive_substring() {
	appDir := s.newProject("", `
[project]
dependencies = ["celerity-sdk-unrelated-tooling"]
`)
	err := RequireCelerityRuntimeSDK(appDir, "python3.13")
	s.Require().Error(err)
}

func (s *SDKDetectTestSuite) Test_python_commented_celerity_sdk_is_ignored() {
	appDir := s.newProject("", `
[project]
# dependencies = ["celerity-sdk>=0.6.0"]
dependencies = ["boto3"]
`)
	err := RequireCelerityRuntimeSDK(appDir, "python3.13")
	s.Require().Error(err)
}

// --- Other runtimes ---

func (s *SDKDetectTestSuite) Test_non_node_non_python_runtime_is_nil() {
	appDir := s.T().TempDir()
	s.NoError(RequireCelerityRuntimeSDK(appDir, "rust1.x"))
}

// --- helpers ---

func (s *SDKDetectTestSuite) newProject(packageJSON, pyprojectTOML string) string {
	appDir := s.T().TempDir()
	if packageJSON != "" {
		s.Require().NoError(os.WriteFile(
			filepath.Join(appDir, "package.json"), []byte(packageJSON), 0644,
		))
	}
	if pyprojectTOML != "" {
		s.Require().NoError(os.WriteFile(
			filepath.Join(appDir, "pyproject.toml"), []byte(pyprojectTOML), 0644,
		))
	}
	return appDir
}

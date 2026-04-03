package devconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type DeployConfigTestSuite struct {
	suite.Suite
}

func TestDeployConfigTestSuite(t *testing.T) {
	suite.Run(t, new(DeployConfigTestSuite))
}

func (s *DeployConfigTestSuite) Test_read_deploy_target_from_clean_json() {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "app.deploy.json")
	err := os.WriteFile(path, []byte(`{"deployTarget": {"name": "aws"}}`), 0o644)
	s.Require().NoError(err)

	target, err := ReadDeployTarget(path)
	s.Require().NoError(err)
	s.Assert().Equal("aws", target)
}

func (s *DeployConfigTestSuite) Test_read_deploy_target_strips_line_comments() {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "app.deploy.jsonc")
	content := `{
  // This is a comment
  "deployTarget": {
    "name": "aws-serverless" // inline comment
  }
}`
	err := os.WriteFile(path, []byte(content), 0o644)
	s.Require().NoError(err)

	target, err := ReadDeployTarget(path)
	s.Require().NoError(err)
	s.Assert().Equal("aws-serverless", target)
}

func (s *DeployConfigTestSuite) Test_read_deploy_target_strips_block_comments() {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "app.deploy.jsonc")
	content := `{
  /* block comment */
  "deployTarget": {
    "name": "gcloud"
  }
}`
	err := os.WriteFile(path, []byte(content), 0o644)
	s.Require().NoError(err)

	target, err := ReadDeployTarget(path)
	s.Require().NoError(err)
	s.Assert().Equal("gcloud", target)
}

func (s *DeployConfigTestSuite) Test_read_deploy_target_strips_trailing_commas() {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "app.deploy.jsonc")
	content := "{\"deployTarget\": {\"name\": \"azure\",\n}}"
	err := os.WriteFile(path, []byte(content), 0o644)
	s.Require().NoError(err)

	target, err := ReadDeployTarget(path)
	s.Require().NoError(err)
	s.Assert().Equal("azure", target)
}

func (s *DeployConfigTestSuite) Test_read_deploy_target_missing_name_returns_error() {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "app.deploy.json")
	err := os.WriteFile(path, []byte(`{"deployTarget": {}}`), 0o644)
	s.Require().NoError(err)

	_, err = ReadDeployTarget(path)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "missing deployTarget.name")
}

func (s *DeployConfigTestSuite) Test_read_deploy_target_invalid_json_returns_error() {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "app.deploy.json")
	err := os.WriteFile(path, []byte(`{invalid}`), 0o644)
	s.Require().NoError(err)

	_, err = ReadDeployTarget(path)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "parsing deploy config")
}

func (s *DeployConfigTestSuite) Test_read_deploy_target_nonexistent_file_returns_error() {
	_, err := ReadDeployTarget("/nonexistent/path")
	s.Assert().Error(err)
}

func (s *DeployConfigTestSuite) Test_find_deploy_config_jsonc_preferred() {
	dir := s.T().TempDir()
	err := os.WriteFile(filepath.Join(dir, "app.deploy.jsonc"), []byte("{}"), 0o644)
	s.Require().NoError(err)
	err = os.WriteFile(filepath.Join(dir, "app.deploy.json"), []byte("{}"), 0o644)
	s.Require().NoError(err)

	result := FindDeployConfig(dir)
	s.Assert().Equal(dir+"/app.deploy.jsonc", result)
}

func (s *DeployConfigTestSuite) Test_find_deploy_config_json_fallback() {
	dir := s.T().TempDir()
	err := os.WriteFile(filepath.Join(dir, "app.deploy.json"), []byte("{}"), 0o644)
	s.Require().NoError(err)

	result := FindDeployConfig(dir)
	s.Assert().Equal(dir+"/app.deploy.json", result)
}

func (s *DeployConfigTestSuite) Test_find_deploy_config_not_found() {
	dir := s.T().TempDir()
	result := FindDeployConfig(dir)
	s.Assert().Equal("", result)
}

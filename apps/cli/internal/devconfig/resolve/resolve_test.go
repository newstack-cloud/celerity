package resolve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ResolveTestSuite struct {
	suite.Suite
}

func TestResolveTestSuite(t *testing.T) {
	suite.Run(t, new(ResolveTestSuite))
}

func (s *ResolveTestSuite) Test_app_dir_empty_returns_cwd() {
	result, err := AppDir("")
	s.Require().NoError(err)
	cwd, _ := os.Getwd()
	s.Assert().Equal(cwd, result)
}

func (s *ResolveTestSuite) Test_app_dir_dot_returns_cwd() {
	result, err := AppDir(".")
	s.Require().NoError(err)
	cwd, _ := os.Getwd()
	s.Assert().Equal(cwd, result)
}

func (s *ResolveTestSuite) Test_app_dir_absolute_path_returned_as_is() {
	dir := s.T().TempDir()
	result, err := AppDir(dir)
	s.Require().NoError(err)
	s.Assert().Equal(dir, result)
}

func (s *ResolveTestSuite) Test_blueprint_path_explicit_flag() {
	dir := s.T().TempDir()
	bp := filepath.Join(dir, "custom.yaml")
	s.Require().NoError(os.WriteFile(bp, []byte("version: 1"), 0o644))

	result, err := BlueprintPath(dir, bp)
	s.Require().NoError(err)
	s.Assert().Equal(bp, result)
}

func (s *ResolveTestSuite) Test_blueprint_path_explicit_relative_flag() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "custom.yaml"), []byte("v: 1"), 0o644))

	result, err := BlueprintPath(dir, "custom.yaml")
	s.Require().NoError(err)
	s.Assert().Equal(filepath.Join(dir, "custom.yaml"), result)
}

func (s *ResolveTestSuite) Test_blueprint_path_explicit_not_found_returns_error() {
	dir := s.T().TempDir()
	_, err := BlueprintPath(dir, "nonexistent.yaml")
	s.Assert().Error(err)
	var nfErr *NotFoundError
	s.Assert().ErrorAs(err, &nfErr)
}

func (s *ResolveTestSuite) Test_blueprint_path_auto_detect_yaml() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "app.blueprint.yaml"), []byte("v: 1"), 0o644))

	result, err := BlueprintPath(dir, "")
	s.Require().NoError(err)
	s.Assert().Equal(filepath.Join(dir, "app.blueprint.yaml"), result)
}

func (s *ResolveTestSuite) Test_blueprint_path_auto_detect_yml() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "app.blueprint.yml"), []byte("v: 1"), 0o644))

	result, err := BlueprintPath(dir, "")
	s.Require().NoError(err)
	s.Assert().Equal(filepath.Join(dir, "app.blueprint.yml"), result)
}

func (s *ResolveTestSuite) Test_blueprint_path_auto_detect_jsonc() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "app.blueprint.jsonc"), []byte("{}"), 0o644))

	result, err := BlueprintPath(dir, "")
	s.Require().NoError(err)
	s.Assert().Equal(filepath.Join(dir, "app.blueprint.jsonc"), result)
}

func (s *ResolveTestSuite) Test_blueprint_path_auto_detect_not_found() {
	dir := s.T().TempDir()
	_, err := BlueprintPath(dir, "")
	s.Assert().Error(err)
}

func (s *ResolveTestSuite) Test_module_path_explicit_flag() {
	result := ModulePath("/app", "custom/module.ts", []string{"src/app.ts"})
	s.Assert().Equal("custom/module.ts", result)
}

func (s *ResolveTestSuite) Test_module_path_auto_detect_first_match() {
	dir := s.T().TempDir()
	s.Require().NoError(os.MkdirAll(filepath.Join(dir, "src"), 0o755))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "src", "app.module.ts"), []byte(""), 0o644))

	result := ModulePath(dir, "", []string{"src/app-module.ts", "src/app.module.ts"})
	s.Assert().Equal("src/app.module.ts", result)
}

func (s *ResolveTestSuite) Test_module_path_no_candidates_found_uses_first() {
	dir := s.T().TempDir()
	result := ModulePath(dir, "", []string{"src/app-module.ts", "src/app.module.ts"})
	s.Assert().Equal("src/app-module.ts", result)
}

func (s *ResolveTestSuite) Test_module_path_empty_candidates() {
	result := ModulePath("/app", "", nil)
	s.Assert().Equal("", result)
}

func (s *ResolveTestSuite) Test_dir_run_mode_uses_local() {
	dir := s.T().TempDir()
	localDir := filepath.Join(dir, "seed", "local")
	s.Require().NoError(os.MkdirAll(localDir, 0o755))

	result := DirWithTestFallback(dir, "seed", "run")
	s.Assert().Equal(localDir, result)
}

func (s *ResolveTestSuite) Test_dir_test_mode_prefers_test() {
	dir := s.T().TempDir()
	s.Require().NoError(os.MkdirAll(filepath.Join(dir, "seed", "local"), 0o755))
	testDir := filepath.Join(dir, "seed", "test")
	s.Require().NoError(os.MkdirAll(testDir, 0o755))

	result := DirWithTestFallback(dir, "seed", "test")
	s.Assert().Equal(testDir, result)
}

func (s *ResolveTestSuite) Test_dir_test_mode_falls_back_to_local() {
	dir := s.T().TempDir()
	localDir := filepath.Join(dir, "config", "local")
	s.Require().NoError(os.MkdirAll(localDir, 0o755))

	result := DirWithTestFallback(dir, "config", "test")
	s.Assert().Equal(localDir, result)
}

func (s *ResolveTestSuite) Test_dir_neither_exists_returns_empty() {
	dir := s.T().TempDir()
	result := DirWithTestFallback(dir, "secrets", "run")
	s.Assert().Equal("", result)
}

func (s *ResolveTestSuite) Test_deploy_target_aws() {
	s.Assert().Equal("aws", DeployTargetToProvider("aws"))
}

func (s *ResolveTestSuite) Test_deploy_target_aws_serverless() {
	s.Assert().Equal("aws", DeployTargetToProvider("aws-serverless"))
}

func (s *ResolveTestSuite) Test_deploy_target_gcloud() {
	s.Assert().Equal("gcp", DeployTargetToProvider("gcloud"))
}

func (s *ResolveTestSuite) Test_deploy_target_gcloud_serverless() {
	s.Assert().Equal("gcp", DeployTargetToProvider("gcloud-serverless"))
}

func (s *ResolveTestSuite) Test_deploy_target_azure() {
	s.Assert().Equal("azure", DeployTargetToProvider("azure"))
}

func (s *ResolveTestSuite) Test_deploy_target_azure_serverless() {
	s.Assert().Equal("azure", DeployTargetToProvider("azure-serverless"))
}

func (s *ResolveTestSuite) Test_deploy_target_unknown_returns_as_is() {
	s.Assert().Equal("custom-target", DeployTargetToProvider("custom-target"))
}

package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type BootstrapNodejsSuite struct {
	suite.Suite
}

func TestBootstrapNodejsSuite(t *testing.T) {
	suite.Run(t, new(BootstrapNodejsSuite))
}

func (s *BootstrapNodejsSuite) Test_Generate_writes_mjs_for_esm_project() {
	appDir := s.newProject(`{"name":"x","type":"module"}`)
	staging := s.T().TempDir()
	bootstrap := &LambdaNodeBootstrap{AppDir: appDir}

	entryPoint, err := bootstrap.Generate(staging)
	s.Require().NoError(err)
	s.Equal("__celerity_lambda_entry__.handler", entryPoint)

	content, err := os.ReadFile(filepath.Join(staging, "__celerity_lambda_entry__.mjs"))
	s.Require().NoError(err)
	s.Contains(string(content), `export { handler } from "@celerity-sdk/serverless-aws/handler"`)

	_, err = os.Stat(filepath.Join(staging, "__celerity_lambda_entry__.js"))
	s.True(os.IsNotExist(err))
}

func (s *BootstrapNodejsSuite) Test_Generate_writes_js_for_cjs_project() {
	appDir := s.newProject(`{"name":"x"}`)
	staging := s.T().TempDir()
	bootstrap := &LambdaNodeBootstrap{AppDir: appDir}

	entryPoint, err := bootstrap.Generate(staging)
	s.Require().NoError(err)
	s.Equal("__celerity_lambda_entry__.handler", entryPoint)

	content, err := os.ReadFile(filepath.Join(staging, "__celerity_lambda_entry__.js"))
	s.Require().NoError(err)
	s.Contains(string(content), `require("@celerity-sdk/serverless-aws/handler")`)
	s.Contains(string(content), "module.exports = { handler }")

	_, err = os.Stat(filepath.Join(staging, "__celerity_lambda_entry__.mjs"))
	s.True(os.IsNotExist(err))
}

func (s *BootstrapNodejsSuite) Test_Generate_writes_js_when_package_json_missing() {
	appDir := s.T().TempDir()
	staging := s.T().TempDir()
	bootstrap := &LambdaNodeBootstrap{AppDir: appDir}

	entryPoint, err := bootstrap.Generate(staging)
	s.Require().NoError(err)
	s.Equal("__celerity_lambda_entry__.handler", entryPoint)

	_, err = os.Stat(filepath.Join(staging, "__celerity_lambda_entry__.js"))
	s.NoError(err)
}

func (s *BootstrapNodejsSuite) Test_Generate_skips_when_user_supplies_mjs() {
	appDir := s.newProject(`{"name":"x","type":"module"}`)
	staging := s.T().TempDir()
	existing := []byte("// user's custom entry\nexport const handler = async () => ({});\n")
	existingPath := filepath.Join(staging, "__celerity_lambda_entry__.mjs")
	s.Require().NoError(os.WriteFile(existingPath, existing, 0644))

	bootstrap := &LambdaNodeBootstrap{AppDir: appDir}
	entryPoint, err := bootstrap.Generate(staging)
	s.Require().NoError(err)
	s.Equal("__celerity_lambda_entry__.handler", entryPoint)

	content, err := os.ReadFile(existingPath)
	s.Require().NoError(err)
	s.Equal(existing, content)
}

func (s *BootstrapNodejsSuite) Test_Generate_skips_when_user_supplies_cjs() {
	appDir := s.newProject(`{"name":"x"}`)
	staging := s.T().TempDir()
	existing := []byte("// user's custom entry\nexports.handler = async () => ({});\n")
	existingPath := filepath.Join(staging, "__celerity_lambda_entry__.js")
	s.Require().NoError(os.WriteFile(existingPath, existing, 0644))

	bootstrap := &LambdaNodeBootstrap{AppDir: appDir}
	_, err := bootstrap.Generate(staging)
	s.Require().NoError(err)

	content, err := os.ReadFile(existingPath)
	s.Require().NoError(err)
	s.Equal(existing, content)
}

func (s *BootstrapNodejsSuite) newProject(packageJSON string) string {
	appDir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(
		filepath.Join(appDir, "package.json"),
		[]byte(packageJSON),
		0644,
	))
	return appDir
}

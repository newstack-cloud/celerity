package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type BootstrapPythonSuite struct {
	suite.Suite
}

func TestBootstrapPythonSuite(t *testing.T) {
	suite.Run(t, new(BootstrapPythonSuite))
}

func (s *BootstrapPythonSuite) Test_Generate_writes_expected_file() {
	staging := s.T().TempDir()
	bootstrap := &LambdaPythonBootstrap{}

	entryPoint, err := bootstrap.Generate(staging)
	s.Require().NoError(err)
	s.Equal("__celerity_lambda_entry__.handler", entryPoint)

	content, err := os.ReadFile(filepath.Join(staging, "__celerity_lambda_entry__.py"))
	s.Require().NoError(err)
	s.Contains(string(content), "from celerity.serverless.aws import create_lambda_handler")
	s.Contains(string(content), "handler = create_lambda_handler()")
}

func (s *BootstrapPythonSuite) Test_Generate_skips_when_file_already_exists() {
	staging := s.T().TempDir()
	existingContent := []byte("# user's own entry file\nhandler = custom_handler\n")
	existingPath := filepath.Join(staging, "__celerity_lambda_entry__.py")
	s.Require().NoError(os.WriteFile(existingPath, existingContent, 0644))

	bootstrap := &LambdaPythonBootstrap{}
	entryPoint, err := bootstrap.Generate(staging)
	s.Require().NoError(err)
	s.Equal("__celerity_lambda_entry__.handler", entryPoint)

	content, err := os.ReadFile(existingPath)
	s.Require().NoError(err)
	s.Equal(existingContent, content)
}

func (s *BootstrapPythonSuite) Test_Generate_fails_when_staging_dir_unwritable() {
	staging := s.T().TempDir()
	s.Require().NoError(os.Chmod(staging, 0500))
	s.T().Cleanup(func() {
		_ = os.Chmod(staging, 0755)
	})

	bootstrap := &LambdaPythonBootstrap{}
	_, err := bootstrap.Generate(staging)
	s.Require().Error(err)
}

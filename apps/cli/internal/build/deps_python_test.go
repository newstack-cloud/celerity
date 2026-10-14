package build

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type DepsPythonSuite struct {
	suite.Suite
}

func TestDepsPythonSuite(t *testing.T) {
	suite.Run(t, new(DepsPythonSuite))
}

func (s *DepsPythonSuite) Test_installAll_fromRequirementsTxt() {
	stub := &stubExec{}
	installer := &PythonDependencyInstaller{Exec: stub}

	appDir := s.T().TempDir()
	targetDir := s.T().TempDir()

	s.Require().NoError(os.WriteFile(
		filepath.Join(appDir, "requirements.txt"), []byte("boto3>=1.34.0\n"), 0644,
	))

	err := installer.InstallDependencies(context.Background(), appDir, targetDir, nil, io.Discard)
	s.Require().NoError(err)

	s.Require().Len(stub.calls, 1)
	call := stub.calls[0]
	s.Equal("pip", call.name)
	s.Contains(call.args, "-r")
	s.Contains(call.args, filepath.Join(appDir, "requirements.txt"))
	s.Contains(call.args, "-t")
	s.Contains(call.args, targetDir)
}

func (s *DepsPythonSuite) Test_installAll_fromPyprojectToml() {
	stub := &stubExec{}
	installer := &PythonDependencyInstaller{Exec: stub}

	appDir := s.T().TempDir()
	targetDir := s.T().TempDir()

	s.Require().NoError(os.WriteFile(
		filepath.Join(appDir, "pyproject.toml"), []byte("[project]\nname = \"test\"\n"), 0644,
	))

	err := installer.InstallDependencies(context.Background(), appDir, targetDir, nil, io.Discard)
	s.Require().NoError(err)

	s.Require().Len(stub.calls, 1)
	call := stub.calls[0]
	s.Equal("pip", call.name)
	s.Contains(call.args, appDir)
	s.Contains(call.args, "-t")
}

func (s *DepsPythonSuite) Test_installAll_noManifest_skips() {
	stub := &stubExec{}
	installer := &PythonDependencyInstaller{Exec: stub}

	appDir := s.T().TempDir()
	targetDir := s.T().TempDir()

	err := installer.InstallDependencies(context.Background(), appDir, targetDir, nil, io.Discard)
	s.NoError(err)
	s.Empty(stub.calls)
}

func (s *DepsPythonSuite) Test_installSubset_invokesCorrectPackages() {
	stub := &stubExec{}
	installer := &PythonDependencyInstaller{Exec: stub}

	targetDir := s.T().TempDir()

	err := installer.InstallDependencies(
		context.Background(), "", targetDir, []string{"boto3", "pydantic"}, io.Discard,
	)
	s.Require().NoError(err)

	s.Require().Len(stub.calls, 1)
	call := stub.calls[0]
	s.Equal("pip", call.name)
	s.Contains(call.args, "boto3")
	s.Contains(call.args, "pydantic")
	s.Contains(call.args, "-t")
}

func (s *DepsPythonSuite) Test_installSubset_emptyList_skips() {
	stub := &stubExec{}
	installer := &PythonDependencyInstaller{Exec: stub}

	err := installer.InstallDependencies(
		context.Background(), "", s.T().TempDir(), []string{}, io.Discard,
	)
	s.NoError(err)
	s.Empty(stub.calls)
}

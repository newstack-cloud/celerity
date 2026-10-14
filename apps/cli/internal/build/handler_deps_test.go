package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type HandlerDepsSuite struct {
	suite.Suite
}

func TestHandlerDepsSuite(t *testing.T) {
	suite.Run(t, new(HandlerDepsSuite))
}

// --- NodeHandlerDependencyReader ---

func (s *HandlerDepsSuite) Test_NodeHandlerDependencyReader_ReadHandlerDependencies_returnsDeps() {
	dir := s.T().TempDir()
	pkgJSON := `{
		"name": "my-app",
		"version": "1.0.0",
		"celerity": {
			"handlerDependencies": {
				"orderHandler": ["stripe", "lodash"],
				"emailHandler": ["nodemailer"]
			}
		}
	}`
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644))

	reader := &NodeHandlerDependencyReader{}
	deps, err := reader.ReadHandlerDependencies(dir)

	s.Require().NoError(err)
	s.Require().NotNil(deps)
	s.Equal([]string{"stripe", "lodash"}, deps["orderHandler"])
	s.Equal([]string{"nodemailer"}, deps["emailHandler"])
}

func (s *HandlerDepsSuite) Test_NodeHandlerDependencyReader_ReadHandlerDependencies_returnsNilWhenSectionAbsent() {
	dir := s.T().TempDir()
	pkgJSON := `{"name": "my-app", "version": "1.0.0"}`
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644))

	reader := &NodeHandlerDependencyReader{}
	deps, err := reader.ReadHandlerDependencies(dir)

	s.NoError(err)
	s.Nil(deps)
}

func (s *HandlerDepsSuite) Test_NodeHandlerDependencyReader_ReadHandlerDependencies_returnsNilWhenFileDoesNotExist() {
	dir := s.T().TempDir()

	reader := &NodeHandlerDependencyReader{}
	deps, err := reader.ReadHandlerDependencies(dir)

	s.NoError(err)
	s.Nil(deps)
}

// --- PythonHandlerDependencyReader ---

func (s *HandlerDepsSuite) Test_PythonHandlerDependencyReader_ReadHandlerDependencies_returnsDeps() {
	dir := s.T().TempDir()
	pyproject := `
[tool.celerity.handlerDependencies]
orderHandler = ["stripe", "requests"]
emailHandler = ["boto3"]
`
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0644))

	reader := &PythonHandlerDependencyReader{}
	deps, err := reader.ReadHandlerDependencies(dir)

	s.Require().NoError(err)
	s.Require().NotNil(deps)
	s.Equal([]string{"stripe", "requests"}, deps["orderHandler"])
	s.Equal([]string{"boto3"}, deps["emailHandler"])
}

func (s *HandlerDepsSuite) Test_PythonHandlerDependencyReader_ReadHandlerDependencies_returnsNilWhenSectionAbsent() {
	dir := s.T().TempDir()
	pyproject := `
[project]
name = "my-app"
version = "1.0.0"
`
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0644))

	reader := &PythonHandlerDependencyReader{}
	deps, err := reader.ReadHandlerDependencies(dir)

	s.NoError(err)
	s.Nil(deps)
}

func (s *HandlerDepsSuite) Test_PythonHandlerDependencyReader_ReadHandlerDependencies_returnsNilWhenFileDoesNotExist() {
	dir := s.T().TempDir()

	reader := &PythonHandlerDependencyReader{}
	deps, err := reader.ReadHandlerDependencies(dir)

	s.NoError(err)
	s.Nil(deps)
}

// --- ReadHandlerDependencies (registry delegation) ---

func (s *HandlerDepsSuite) Test_ReadHandlerDependencies_delegatesToNodeReader() {
	dir := s.T().TempDir()
	pkgJSON := `{
		"name": "my-app",
		"celerity": {
			"handlerDependencies": {
				"handler1": ["express"]
			}
		}
	}`
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644))

	registry := NewRegistry()
	registry.RegisterHandlerDependencyReader("nodejs", &NodeHandlerDependencyReader{})

	deps, err := ReadHandlerDependencies(registry, "nodejs24.x", dir)

	s.Require().NoError(err)
	s.Require().NotNil(deps)
	s.Equal([]string{"express"}, deps["handler1"])
}

func (s *HandlerDepsSuite) Test_ReadHandlerDependencies_delegatesToPythonReader() {
	dir := s.T().TempDir()
	pyproject := `
[tool.celerity.handlerDependencies]
handler1 = ["boto3"]
`
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0644))

	registry := NewRegistry()
	registry.RegisterHandlerDependencyReader("python", &PythonHandlerDependencyReader{})

	deps, err := ReadHandlerDependencies(registry, "python3.12", dir)

	s.Require().NoError(err)
	s.Require().NotNil(deps)
	s.Equal([]string{"boto3"}, deps["handler1"])
}

func (s *HandlerDepsSuite) Test_ReadHandlerDependencies_returnsNilWhenNoReaderRegistered() {
	registry := NewRegistry()

	deps, err := ReadHandlerDependencies(registry, "rust1.x", "/tmp/nodir")

	s.NoError(err)
	s.Nil(deps)
}

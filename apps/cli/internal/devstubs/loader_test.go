package devstubs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type LoaderTestSuite struct {
	suite.Suite
}

func TestLoaderTestSuite(t *testing.T) {
	suite.Run(t, new(LoaderTestSuite))
}

func (s *LoaderTestSuite) mkdirAll(path string) {
	s.Require().NoError(os.MkdirAll(path, 0o755))
}

func (s *LoaderTestSuite) writeFile(path string, content string) {
	s.Require().NoError(os.WriteFile(path, []byte(content), 0o644))
}

const stubEndpointYAML = `endpoint:
  method: GET
  path: /charges
stubs:
  - name: success
    responses:
      - is:
          statusCode: 200
          body: []
`

const minimalStubYAML = "endpoint:\n  method: GET\n  path: /\nstubs:\n  - responses:\n      - is:\n          statusCode: 200\n"

func (s *LoaderTestSuite) Test_no_stubs_dir_returns_nil() {
	appDir := s.T().TempDir()
	services, err := LoadStubs(appDir)
	s.Require().NoError(err)
	s.Assert().Nil(services)
}

func (s *LoaderTestSuite) Test_loads_service_with_endpoint() {
	appDir := s.T().TempDir()
	svcDir := filepath.Join(appDir, "stubs", "payments")
	s.mkdirAll(svcDir)

	s.writeFile(filepath.Join(svcDir, "service.yaml"), `
port: 9001
configKey: payments_base_url
`)

	s.writeFile(filepath.Join(svcDir, "get-charges.yaml"), stubEndpointYAML)

	services, err := LoadStubs(appDir)
	s.Require().NoError(err)
	s.Require().Len(services, 1)
	s.Assert().Equal("payments", services[0].Name)
	s.Assert().Equal(9001, services[0].Config.Port)
	s.Assert().Equal("payments_base_url", services[0].Config.ConfigKey)
	s.Require().Len(services[0].Endpoints, 1)
	s.Assert().Equal("GET", services[0].Endpoints[0].Endpoint.Method)
	s.Assert().Equal("/charges", services[0].Endpoints[0].Endpoint.Path)
}

func (s *LoaderTestSuite) Test_loads_multiple_services() {
	appDir := s.T().TempDir()

	for _, svc := range []string{"payments", "shipping"} {
		svcDir := filepath.Join(appDir, "stubs", svc)
		s.mkdirAll(svcDir)
		s.writeFile(filepath.Join(svcDir, "service.yaml"), "port: 9001\nconfigKey: "+svc+"_url\n")
		s.writeFile(filepath.Join(svcDir, "get.yaml"), minimalStubYAML)
	}

	services, err := LoadStubs(appDir)
	s.Require().NoError(err)
	s.Assert().Len(services, 2)
}

func (s *LoaderTestSuite) Test_skips_non_yaml_files_in_service_dir() {
	appDir := s.T().TempDir()
	svcDir := filepath.Join(appDir, "stubs", "payments")
	s.mkdirAll(svcDir)

	s.writeFile(filepath.Join(svcDir, "service.yaml"), "port: 9001\nconfigKey: key\n")
	s.writeFile(filepath.Join(svcDir, "readme.txt"), "not an endpoint")
	s.writeFile(filepath.Join(svcDir, "get-charges.yaml"), minimalStubYAML)

	services, err := LoadStubs(appDir)
	s.Require().NoError(err)
	s.Require().Len(services, 1)
	s.Assert().Len(services[0].Endpoints, 1)
}

func (s *LoaderTestSuite) Test_supports_yml_extension() {
	appDir := s.T().TempDir()
	svcDir := filepath.Join(appDir, "stubs", "payments")
	s.mkdirAll(svcDir)

	s.writeFile(filepath.Join(svcDir, "service.yaml"), "port: 9001\nconfigKey: key\n")
	s.writeFile(filepath.Join(svcDir, "get-charges.yml"), minimalStubYAML)

	services, err := LoadStubs(appDir)
	s.Require().NoError(err)
	s.Require().Len(services, 1)
	s.Assert().Len(services[0].Endpoints, 1)
}

func (s *LoaderTestSuite) Test_skips_non_directory_entries_in_stubs_dir() {
	appDir := s.T().TempDir()
	stubsDir := filepath.Join(appDir, "stubs")
	s.mkdirAll(stubsDir)

	s.writeFile(filepath.Join(stubsDir, "readme.md"), "stubs info")

	svcDir := filepath.Join(stubsDir, "payments")
	s.mkdirAll(svcDir)
	s.writeFile(filepath.Join(svcDir, "service.yaml"), "port: 9001\nconfigKey: key\n")
	s.writeFile(filepath.Join(svcDir, "get.yaml"), minimalStubYAML)

	services, err := LoadStubs(appDir)
	s.Require().NoError(err)
	s.Assert().Len(services, 1)
}

func (s *LoaderTestSuite) Test_missing_service_yaml_returns_error() {
	appDir := s.T().TempDir()
	svcDir := filepath.Join(appDir, "stubs", "payments")
	s.mkdirAll(svcDir)

	_, err := LoadStubs(appDir)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "service.yaml")
}

func (s *LoaderTestSuite) Test_invalid_service_yaml_returns_error() {
	appDir := s.T().TempDir()
	svcDir := filepath.Join(appDir, "stubs", "payments")
	s.mkdirAll(svcDir)

	s.writeFile(filepath.Join(svcDir, "service.yaml"), ": invalid: yaml: [")

	_, err := LoadStubs(appDir)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "parsing service.yaml")
}

func (s *LoaderTestSuite) Test_invalid_endpoint_yaml_returns_error() {
	appDir := s.T().TempDir()
	svcDir := filepath.Join(appDir, "stubs", "payments")
	s.mkdirAll(svcDir)

	s.writeFile(filepath.Join(svcDir, "service.yaml"), "port: 9001\nconfigKey: key\n")
	s.writeFile(filepath.Join(svcDir, "bad.yaml"), ": invalid: yaml: [")

	_, err := LoadStubs(appDir)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "parsing bad.yaml")
}

func (s *LoaderTestSuite) Test_empty_stubs_dir_returns_empty_slice() {
	appDir := s.T().TempDir()
	s.mkdirAll(filepath.Join(appDir, "stubs"))

	services, err := LoadStubs(appDir)
	s.Require().NoError(err)
	s.Assert().Empty(services)
}

func (s *LoaderTestSuite) Test_loads_default_response_from_service_config() {
	appDir := s.T().TempDir()
	svcDir := filepath.Join(appDir, "stubs", "payments")
	s.mkdirAll(svcDir)

	s.writeFile(filepath.Join(svcDir, "service.yaml"), `
port: 9001
configKey: payments_url
defaultResponse:
  headers:
    Content-Type: application/json
`)
	s.writeFile(filepath.Join(svcDir, "get.yaml"), minimalStubYAML)

	services, err := LoadStubs(appDir)
	s.Require().NoError(err)
	s.Require().Len(services, 1)
	s.Assert().Equal(
		map[string]any{"headers": map[string]any{"Content-Type": "application/json"}},
		services[0].Config.DefaultResponse,
	)
}

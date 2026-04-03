package blueprint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/suite"
)

type HelpersTestSuite struct {
	suite.Suite
}

func (s *HelpersTestSuite) loadBlueprint(yamlContent string) *schema.Blueprint {
	bp, err := schema.LoadString(yamlContent, schema.YAMLSpecFormat)
	s.Require().NoError(err, "failed to load test blueprint")
	return bp
}

func (s *HelpersTestSuite) Test_LoadForDev_yaml_format() {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "app.blueprint.yaml")
	content := `
version: 2025-11-02
resources: {}
`
	s.Require().NoError(os.WriteFile(path, []byte(content), 0o644))

	bp, format, err := LoadForDev(path)
	s.Require().NoError(err)
	s.Assert().NotNil(bp)
	s.Assert().Equal(schema.YAMLSpecFormat, format)
}

func (s *HelpersTestSuite) Test_LoadForDev_jsonc_format() {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "app.blueprint.jsonc")
	content := `{
  "version": "2025-11-02",
  "resources": {}
}`
	s.Require().NoError(os.WriteFile(path, []byte(content), 0o644))

	bp, format, err := LoadForDev(path)
	s.Require().NoError(err)
	s.Assert().NotNil(bp)
	s.Assert().Equal(schema.JWCCSpecFormat, format)
}

func (s *HelpersTestSuite) Test_DetectRuntime_finds_nodejs_runtime() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myHandler:
    type: "celerity/handler"
    spec:
      handlerName: hello
      runtime: nodejs24.x
      codeLocation: handler.ts
      handler: HelloHandler.handle
`)
	runtime, err := DetectRuntime(bp)
	s.Require().NoError(err)
	s.Assert().Equal("nodejs24.x", runtime)
}

func (s *HelpersTestSuite) Test_DetectRuntime_falls_back_to_handlerConfig() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myHandler:
    type: "celerity/handler"
    spec:
      handler: handlers.hello
    linkSelector:
      byLabel:
        handlerGroup: api
  apiHandlerConfig:
    type: "celerity/handlerConfig"
    metadata:
      labels:
        handlerGroup: api
    spec:
      runtime: python3.13
      codeLocation: ./handlers
`)
	runtime, err := DetectRuntime(bp)
	s.Require().NoError(err)
	s.Assert().Equal("python3.13", runtime)
}

func (s *HelpersTestSuite) Test_DetectRuntime_falls_back_to_shared_metadata() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myHandler:
    type: "celerity/handler"
    spec:
      handler: handlers.hello
metadata:
  sharedHandlerConfig:
    runtime: nodejs24.x
    codeLocation: ./handlers
`)
	runtime, err := DetectRuntime(bp)
	s.Require().NoError(err)
	s.Assert().Equal("nodejs24.x", runtime)
}

func (s *HelpersTestSuite) Test_DetectRuntime_prefers_handler_over_handlerConfig() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myHandler:
    type: "celerity/handler"
    spec:
      handler: handlers.hello
      runtime: nodejs24.x
  apiHandlerConfig:
    type: "celerity/handlerConfig"
    metadata:
      labels:
        handlerGroup: api
    spec:
      runtime: python3.13
`)
	runtime, err := DetectRuntime(bp)
	s.Require().NoError(err)
	s.Assert().Equal("nodejs24.x", runtime)
}

func (s *HelpersTestSuite) Test_DetectRuntime_errors_on_no_runtime_anywhere() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myTable:
    type: "celerity/datastore"
    spec:
      name: users
`)
	_, err := DetectRuntime(bp)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "no runtime found")
}

func (s *HelpersTestSuite) Test_ResolveRuntimeImage_nodejs22() {
	image, err := ResolveRuntimeImage("nodejs24.x", "0.3.0")
	s.Require().NoError(err)
	s.Assert().Equal("ghcr.io/newstack-cloud/celerity-runtime-nodejs-24:dev-0.3.0", image)
}

func (s *HelpersTestSuite) Test_ResolveRuntimeImage_python313() {
	image, err := ResolveRuntimeImage("python3.13", "0.1.0")
	s.Require().NoError(err)
	s.Assert().Equal("ghcr.io/newstack-cloud/celerity-runtime-python-3-13:dev-0.1.0", image)
}

func (s *HelpersTestSuite) Test_ResolveRuntimeImage_unsupported_runtime() {
	_, err := ResolveRuntimeImage("ruby3.3", "0.3.0")
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "unsupported runtime")
}

func (s *HelpersTestSuite) Test_CollectHandlerInfo_extracts_http_handlers() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  helloHandler:
    type: "celerity/handler"
    metadata:
      annotations:
        celerity.handler.http.method: GET
        celerity.handler.http.path: /hello
    spec:
      handlerName: Test-Hello-v1
      runtime: nodejs24.x
      codeLocation: handler.ts
      handler: HelloHandler.handle
  ordersHandler:
    type: "celerity/handler"
    metadata:
      annotations:
        celerity.handler.http.method: POST
        celerity.handler.http.path: /orders
    spec:
      handlerName: Create-Order-v1
      runtime: nodejs24.x
      codeLocation: orders.ts
      handler: OrdersHandler.create
`)
	handlers := CollectHandlerInfo(bp)
	s.Require().Len(handlers, 2)

	// Build a lookup by handler name for stable assertion (map iteration is random).
	byName := map[string]HandlerInfo{}
	for _, h := range handlers {
		byName[h.HandlerName] = h
	}

	hello := byName["Test-Hello-v1"]
	s.Assert().Equal("GET", hello.Method)
	s.Assert().Equal("/hello", hello.Path)
	s.Assert().Equal("http", hello.HandlerType)
	s.Assert().Equal("nodejs24.x", hello.Runtime)

	orders := byName["Create-Order-v1"]
	s.Assert().Equal("POST", orders.Method)
	s.Assert().Equal("/orders", orders.Path)
	s.Assert().Equal("http", orders.HandlerType)
}

func (s *HelpersTestSuite) Test_CollectHandlerInfo_empty_blueprint() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources: {}
`)
	handlers := CollectHandlerInfo(bp)
	s.Assert().Empty(handlers)
}

func (s *HelpersTestSuite) Test_CollectHandlerInfo_skips_non_handler_resources() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
  helloHandler:
    type: "celerity/handler"
    metadata:
      annotations:
        celerity.handler.http.method: GET
        celerity.handler.http.path: /hello
    spec:
      handlerName: Hello
      runtime: nodejs24.x
`)
	handlers := CollectHandlerInfo(bp)
	s.Require().Len(handlers, 1)
	s.Assert().Equal("Hello", handlers[0].HandlerName)
}

func (s *HelpersTestSuite) Test_DetectRuntimeFromProject_finds_nodejs_from_package_json() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0o644))

	runtime, err := DetectRuntimeFromProject(dir)
	s.Require().NoError(err)
	s.Assert().Equal("nodejs24.x", runtime)
}

func (s *HelpersTestSuite) Test_DetectRuntimeFromProject_finds_python_from_pyproject_toml() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(`[project]`), 0o644))

	runtime, err := DetectRuntimeFromProject(dir)
	s.Require().NoError(err)
	s.Assert().Equal("python3.13", runtime)
}

func (s *HelpersTestSuite) Test_DetectRuntimeFromProject_prefers_nodejs_when_both_exist() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0o644))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(`[project]`), 0o644))

	runtime, err := DetectRuntimeFromProject(dir)
	s.Require().NoError(err)
	s.Assert().Equal("nodejs24.x", runtime)
}

func (s *HelpersTestSuite) Test_DetectRuntimeFromProject_errors_on_no_project_files() {
	dir := s.T().TempDir()

	_, err := DetectRuntimeFromProject(dir)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "cannot detect runtime")
}

func TestHelpersTestSuite(t *testing.T) {
	suite.Run(t, new(HelpersTestSuite))
}

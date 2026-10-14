package handlerresolve

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type ResolverTestSuite struct {
	suite.Suite
	logger *zap.Logger
}

func TestResolverTestSuite(t *testing.T) {
	suite.Run(t, new(ResolverTestSuite))
}

func (s *ResolverTestSuite) SetupTest() {
	s.logger = zap.NewNop()
}

// stubExtractor lets us inject a fake extraction result without depending
// on the celerity-extract binary being installed.
type stubExtractor struct {
	manifest *preprocess.HandlerManifest
	err      error
}

func (e *stubExtractor) Extract(_ context.Context) (*preprocess.HandlerManifest, error) {
	if e.err != nil {
		return nil, e.err
	}
	return e.manifest, nil
}

func (s *ResolverTestSuite) stubFactory(extractor Extractor) ExtractorFactory {
	return func(_ preprocess.ExtractorConfig, _ *zap.Logger) Extractor {
		return extractor
	}
}

func (s *ResolverTestSuite) Test_Resolve_decorator_driven_node_project_returns_handlers_from_extractor() {
	appDir := s.T().TempDir()
	s.writeFile(filepath.Join(appDir, "package.json"), `{"name":"test","type":"module","dependencies":{"@celerity-sdk/core":"^0.6.0"}}`)
	s.writeFile(filepath.Join(appDir, "src", "app-module.ts"), "export default class AppModule {}")

	blueprintPath := filepath.Join(appDir, "app.blueprint.yaml")
	s.writeFile(blueprintPath, decoratorDrivenBlueprint)

	extractor := &stubExtractor{
		manifest: &preprocess.HandlerManifest{
			Version: "1",
			Handlers: []preprocess.ClassHandler{
				{
					ResourceName: "getUser",
					ClassName:    "UsersController",
					MethodName:   "getUser",
					SourceFile:   "src/users/users-controller.ts",
					HandlerType:  "http",
					Annotations: map[string]any{
						"celerity.handler.http.method": "GET",
						"celerity.handler.http.path":   "/users/{id}",
					},
					Spec: preprocess.HandlerSpec{
						HandlerName:  "users-controller.getUser",
						CodeLocation: "src/users/users-controller.ts",
						Handler:      "getUser",
					},
				},
			},
		},
	}

	resolved, err := resolveWith(
		context.Background(),
		Options{AppDir: appDir, BlueprintFile: blueprintPath},
		s.stubFactory(extractor),
		s.logger,
	)
	s.Require().NoError(err)
	s.Require().NotNil(resolved)
	s.Equal("nodejs24.x", resolved.Runtime)
	s.Require().Len(resolved.Handlers, 1)
	s.Equal("getUser", resolved.Handlers[0].ResourceName)
}

func (s *ResolverTestSuite) Test_Resolve_blueprint_driven_project_returns_handlers_from_blueprint() {
	appDir := s.T().TempDir()
	s.writeFile(filepath.Join(appDir, "package.json"), `{"name":"test","dependencies":{"@celerity-sdk/core":"^0.6.0"}}`)

	blueprintPath := filepath.Join(appDir, "app.blueprint.yaml")
	s.writeFile(blueprintPath, blueprintWithExplicitHandler)

	extractor := &stubExtractor{
		manifest: &preprocess.HandlerManifest{Version: "1"},
	}

	resolved, err := resolveWith(
		context.Background(),
		Options{AppDir: appDir, BlueprintFile: blueprintPath},
		s.stubFactory(extractor),
		s.logger,
	)
	s.Require().NoError(err)
	s.Require().Len(resolved.Handlers, 1)
	s.Equal("getUser", resolved.Handlers[0].ResourceName)
	s.Equal("users.getUser", resolved.Handlers[0].HandlerName)
}

func (s *ResolverTestSuite) Test_Resolve_fails_when_blueprint_missing() {
	resolved, err := resolveWith(
		context.Background(),
		Options{AppDir: s.T().TempDir(), BlueprintFile: "does-not-exist.yaml"},
		s.stubFactory(&stubExtractor{}),
		s.logger,
	)
	s.Nil(resolved)
	s.Require().Error(err)
	s.Contains(err.Error(), "loading blueprint")
}

func (s *ResolverTestSuite) Test_Resolve_fails_when_runtime_cannot_be_detected() {
	appDir := s.T().TempDir()
	// No package.json, no pyproject.toml → runtime detection fails
	blueprintPath := filepath.Join(appDir, "app.blueprint.yaml")
	s.writeFile(blueprintPath, minimalBlueprint)

	resolved, err := resolveWith(
		context.Background(),
		Options{AppDir: appDir, BlueprintFile: blueprintPath},
		s.stubFactory(&stubExtractor{}),
		s.logger,
	)
	s.Nil(resolved)
	s.Require().Error(err)
	s.Contains(err.Error(), "detecting runtime")
}

func (s *ResolverTestSuite) Test_Resolve_propagates_extractor_errors() {
	appDir := s.T().TempDir()
	s.writeFile(filepath.Join(appDir, "package.json"), `{"name":"test","type":"module"}`)
	blueprintPath := filepath.Join(appDir, "app.blueprint.yaml")
	s.writeFile(blueprintPath, decoratorDrivenBlueprint)

	extractor := &stubExtractor{err: errFromExtractor}

	resolved, err := resolveWith(
		context.Background(),
		Options{AppDir: appDir, BlueprintFile: blueprintPath},
		s.stubFactory(extractor),
		s.logger,
	)
	s.Nil(resolved)
	s.Require().Error(err)
	s.Contains(err.Error(), "handler extraction")
}

func (s *ResolverTestSuite) writeFile(path, content string) {
	s.Require().NoError(os.MkdirAll(filepath.Dir(path), 0755))
	s.Require().NoError(os.WriteFile(path, []byte(content), 0644))
}

var errFromExtractor = extractorErr("boom")

type extractorErr string

func (e extractorErr) Error() string { return string(e) }

const decoratorDrivenBlueprint = `version: 2025-11-02
resources:
  myApi:
    type: "celerity/api"
    spec:
      protocols:
        - http
`

const blueprintWithExplicitHandler = `version: 2025-11-02
resources:
  getUser:
    type: "celerity/handler"
    spec:
      runtime: "nodejs24.x"
      handlerName: "users.getUser"
      codeLocation: "src/users.ts"
`

const minimalBlueprint = `version: 2025-11-02
resources:
  something:
    type: "celerity/api"
    spec:
      protocols:
        - http
`

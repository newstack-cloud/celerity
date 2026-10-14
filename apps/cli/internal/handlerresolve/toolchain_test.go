package handlerresolve

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/consts"
	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// stubToolchain records what it was asked for and answers with what the test
// set up.
type stubToolchain struct {
	prepared     []handlertoolchain.PrepareParams
	extractCalls int
	manifest     *preprocess.HandlerManifest
	prepareErr   error
	extractErr   error
	validateErr  error
}

func (t *stubToolchain) Name() string { return "stub" }

func (t *stubToolchain) Validate(string) error { return t.validateErr }

func (t *stubToolchain) Prepare(_ context.Context, p handlertoolchain.PrepareParams) error {
	t.prepared = append(t.prepared, p)
	return t.prepareErr
}

func (t *stubToolchain) Extract(
	context.Context, handlertoolchain.ExtractParams,
) (*preprocess.HandlerManifest, error) {
	t.extractCalls++
	return t.manifest, t.extractErr
}

func (t *stubToolchain) BuildBinary(context.Context, handlertoolchain.BinaryParams) error {
	return nil
}

func (t *stubToolchain) WatchPaths() handlertoolchain.WatchPaths {
	return handlertoolchain.WatchPaths{}
}

func (t *stubToolchain) TestCommand(string) ([]string, bool) { return nil, false }

type ToolchainResolveSuite struct {
	suite.Suite
	appDir        string
	blueprintFile string
	logger        *zap.Logger
}

func (s *ToolchainResolveSuite) SetupTest() {
	s.appDir = s.T().TempDir()
	s.logger = zap.NewNop()
	s.blueprintFile = filepath.Join(s.appDir, "app.blueprint.yaml")
	s.Require().NoError(os.WriteFile(s.blueprintFile, []byte(`
version: 2025-11-02
transform: celerity-2026-02-28
resources:
  ordersTable:
    type: "celerity/datastore"
    spec:
      name: orders
  primaryDb:
    type: "celerity/sqlDatabase"
    spec:
      engine: postgres
  getOrder:
    type: "celerity/handler"
    metadata:
      annotations:
        celerity.handler.http: true
        celerity.handler.http.method: "GET"
        celerity.handler.http.path: "/orders/{orderId}"
    spec:
      handlerName: Orders-GetOrder-v1
      handler: getOrder
      runtime: "os-only.debian13"
`), 0o644))
}

func (s *ToolchainResolveSuite) options(tc handlertoolchain.Toolchain) Options {
	return Options{
		AppDir:        s.appDir,
		BlueprintFile: s.blueprintFile,
		Toolchain:     tc,
		DeployTarget:  "aws-serverless",
		Mode:          handlertoolchain.ModeDeploy,
	}
}

// A compiled language cannot discover its resources at startup, so the kinds
// the blueprint declares have to reach the toolchain that links them.
func (s *ToolchainResolveSuite) Test_prepare_receives_the_blueprints_resources_and_engines() {
	tc := &stubToolchain{manifest: &preprocess.HandlerManifest{Version: "v1"}}

	_, err := Resolve(context.Background(), s.options(tc), s.logger)
	s.Require().NoError(err)

	s.Require().Len(tc.prepared, 1)
	s.Assert().Equal([]string{"datastore", "sqlDatabase"}, tc.prepared[0].ResourceKinds)
	s.Assert().Equal([]string{"postgres"}, tc.prepared[0].SQLEngines)
	s.Assert().Equal("aws-serverless", tc.prepared[0].DeployTarget)
	s.Assert().Equal(handlertoolchain.ModeDeploy, tc.prepared[0].Mode)
}

// Extraction compiles the application, so generation has to come first or it
// compiles against the wrong platform imports, or none.
func (s *ToolchainResolveSuite) Test_prepare_runs_before_extraction() {
	tc := &stubToolchain{prepareErr: errors.New("generate failed")}

	_, err := Resolve(context.Background(), s.options(tc), s.logger)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "preparing the stub toolchain")
	s.Assert().Zero(tc.extractCalls, "extraction should not run after generation failed")
}

func (s *ToolchainResolveSuite) Test_the_toolchains_manifest_is_the_one_merged() {
	tc := &stubToolchain{manifest: &preprocess.HandlerManifest{
		Version: "v1",
		FunctionHandlers: []preprocess.FunctionHandler{{
			ResourceName: "listOrders",
			Annotations: map[string]any{
				"celerity.handler.http":        true,
				"celerity.handler.http.method": "GET",
				"celerity.handler.http.path":   "/orders",
			},
			Spec: preprocess.HandlerSpec{HandlerName: "Orders-ListOrders-v1", Handler: "listOrders"},
		}},
	}}

	resolved, err := Resolve(context.Background(), s.options(tc), s.logger)
	s.Require().NoError(err)
	s.Assert().Equal(1, tc.extractCalls)
	s.Assert().Len(resolved.Manifest.FunctionHandlers, 1)
	s.Assert().Contains(resolved.MergedBlueprint.Resources.Values, "listOrders")
}

// A toolchain that cannot extract is not an error: the blueprint's
// hand-authored handler resources stand on their own.
func (s *ToolchainResolveSuite) Test_a_toolchain_that_does_not_extract_falls_back_to_the_blueprint() {
	tc := &stubToolchain{manifest: nil}

	resolved, err := Resolve(context.Background(), s.options(tc), s.logger)
	s.Require().NoError(err)
	s.Assert().Empty(resolved.Manifest.AllHandlers())
	// The blueprint's own handler survives the merge.
	s.Assert().Contains(resolved.MergedBlueprint.Resources.Values, "getOrder")
}

func (s *ToolchainResolveSuite) Test_the_resolved_toolchain_is_handed_back_to_the_caller() {
	tc := &stubToolchain{manifest: &preprocess.HandlerManifest{Version: "v1"}}

	resolved, err := Resolve(context.Background(), s.options(tc), s.logger)
	s.Require().NoError(err)
	s.Assert().Same(tc, resolved.Toolchain)
}

func (s *ToolchainResolveSuite) Test_RequireRuntimeSDK_defers_to_the_toolchain() {
	tc := &stubToolchain{validateErr: errors.New("go.mod does not require the SDK")}

	err := RequireRuntimeSDK(s.appDir, &ResolvedHandlers{
		Runtime:   consts.RuntimeOSOnlyDebian13,
		Toolchain: tc,
	})
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "go.mod")
}

// Without a toolchain it is the language SDK check as before.
func (s *ToolchainResolveSuite) Test_RequireRuntimeSDK_without_a_toolchain_checks_the_language_sdk() {
	err := RequireRuntimeSDK(s.appDir, &ResolvedHandlers{Runtime: "nodejs24.x"})
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "package.json")
}

func TestToolchainResolveSuite(t *testing.T) { suite.Run(t, new(ToolchainResolveSuite)) }

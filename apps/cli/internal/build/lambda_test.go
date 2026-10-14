package build

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// --- mock implementations ---

type installCall struct {
	appDir    string
	targetDir string
	packages  []string
}

type mockInstaller struct {
	installed []installCall
}

func (m *mockInstaller) InstallDependencies(_ context.Context, appDir, targetDir string, packages []string, _ io.Writer) error {
	m.installed = append(m.installed, installCall{appDir, targetDir, packages})
	// Write a dummy file so the zip layer is not empty.
	return os.WriteFile(filepath.Join(targetDir, "dummy.js"), []byte("module.exports = {}"), 0644)
}

type mockCompiler struct {
	compileCalled bool
}

func (m *mockCompiler) Compile(_ context.Context, srcDir, outDir string, _ io.Writer) error {
	m.compileCalled = true
	// Copy .js files from srcDir to outDir to simulate compilation.
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".js") {
			data, readErr := os.ReadFile(filepath.Join(srcDir, e.Name()))
			if readErr != nil {
				return readErr
			}
			if writeErr := os.WriteFile(filepath.Join(outDir, e.Name()), data, 0644); writeErr != nil {
				return writeErr
			}
		}
	}
	return nil
}

// --- test suite ---

type LambdaSuite struct {
	suite.Suite
}

func TestLambdaSuite(t *testing.T) {
	suite.Run(t, new(LambdaSuite))
}

func (s *LambdaSuite) newTestRegistry(installer DependencyInstaller, compiler SourceCompiler) *Registry {
	r := NewRegistry()
	r.RegisterDependencyInstaller("nodejs", installer)
	if compiler != nil {
		r.RegisterSourceCompiler("nodejs", compiler)
	}
	return r
}

func (s *LambdaSuite) newBuildParams(appDir, outputDir string, handlers []HandlerInfo, handlerDeps map[string][]string) *BuildParams {
	return &BuildParams{
		AppDir:              appDir,
		Runtime:             "nodejs24.x",
		DeployTarget:        "aws-serverless",
		Handlers:            handlers,
		HandlerDependencies: handlerDeps,
		OutputDir:           outputDir,
		Logger:              zap.NewNop(),
		ToolOutput:          io.Discard,
	}
}

func (s *LambdaSuite) setupAppDir() string {
	dir := s.T().TempDir()
	// Write a simple JS file to act as handler source.
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "handler.js"),
		[]byte("exports.handler = async () => ({ statusCode: 200 })"),
		0644,
	))
	return dir
}

func (s *LambdaSuite) Test_Build_noHandlerDependencies_sharedLayerBuiltNoPerHandlerLayers() {
	inst := &mockInstaller{}
	reg := s.newTestRegistry(inst, nil)
	strategy := &LambdaStrategy{Registry: reg}

	appDir := s.setupAppDir()
	outputDir := s.T().TempDir()

	handlers := []HandlerInfo{
		{ResourceName: "handlerA", HandlerName: "handler.handler", CodeLocation: "handler.js"},
		{ResourceName: "handlerB", HandlerName: "handler.handler", CodeLocation: "handler.js"},
	}
	params := s.newBuildParams(appDir, outputDir, handlers, nil)

	manifest, err := strategy.Build(context.Background(), params)

	s.Require().NoError(err)
	s.NotNil(manifest.Lambda.SharedLayer, "shared layer should be built when there are no per-handler deps")
	s.Nil(manifest.Handlers["handlerA"].Lambda.Dependencies, "handlerA should not have a custom layer")
	s.Nil(manifest.Handlers["handlerB"].Lambda.Dependencies, "handlerB should not have a custom layer")
}

func (s *LambdaSuite) Test_Build_handlerDepsForSubset_sharedLayerPlusCustomLayers() {
	inst := &mockInstaller{}
	reg := s.newTestRegistry(inst, nil)
	strategy := &LambdaStrategy{Registry: reg}

	appDir := s.setupAppDir()
	outputDir := s.T().TempDir()

	handlers := []HandlerInfo{
		{ResourceName: "handlerA", HandlerName: "handler.handler", CodeLocation: "handler.js"},
		{ResourceName: "handlerB", HandlerName: "handler.handler", CodeLocation: "handler.js"},
	}
	handlerDeps := map[string][]string{
		"handlerB": {"stripe"},
	}
	params := s.newBuildParams(appDir, outputDir, handlers, handlerDeps)

	manifest, err := strategy.Build(context.Background(), params)

	s.Require().NoError(err)
	s.NotNil(manifest.Lambda.SharedLayer, "shared layer should be built for handlers without custom deps")
	s.Nil(manifest.Handlers["handlerA"].Lambda.Dependencies, "handlerA uses shared layer")
	s.NotNil(manifest.Handlers["handlerB"].Lambda.Dependencies, "handlerB should have a custom layer")
}

func (s *LambdaSuite) Test_Build_handlerDepsForAllHandlers_noSharedLayer() {
	inst := &mockInstaller{}
	reg := s.newTestRegistry(inst, nil)
	strategy := &LambdaStrategy{Registry: reg}

	appDir := s.setupAppDir()
	outputDir := s.T().TempDir()

	handlers := []HandlerInfo{
		{ResourceName: "handlerA", HandlerName: "handler.handler", CodeLocation: "handler.js"},
		{ResourceName: "handlerB", HandlerName: "handler.handler", CodeLocation: "handler.js"},
	}
	handlerDeps := map[string][]string{
		"handlerA": {"express"},
		"handlerB": {"stripe"},
	}
	params := s.newBuildParams(appDir, outputDir, handlers, handlerDeps)

	manifest, err := strategy.Build(context.Background(), params)

	s.Require().NoError(err)
	s.Nil(manifest.Lambda.SharedLayer, "no shared layer when all handlers have custom deps")
	s.NotNil(manifest.Handlers["handlerA"].Lambda.Dependencies)
	s.NotNil(manifest.Handlers["handlerB"].Lambda.Dependencies)
}

func (s *LambdaSuite) Test_Build_deduplication_identicalDepsSetsShareOneLayerArtifact() {
	inst := &mockInstaller{}
	reg := s.newTestRegistry(inst, nil)
	strategy := &LambdaStrategy{Registry: reg}

	appDir := s.setupAppDir()
	outputDir := s.T().TempDir()

	handlers := []HandlerInfo{
		{ResourceName: "handlerA", HandlerName: "handler.handler", CodeLocation: "handler.js"},
		{ResourceName: "handlerB", HandlerName: "handler.handler", CodeLocation: "handler.js"},
	}
	// Both handlers declare the same deps (order may differ).
	handlerDeps := map[string][]string{
		"handlerA": {"stripe", "lodash"},
		"handlerB": {"lodash", "stripe"},
	}
	params := s.newBuildParams(appDir, outputDir, handlers, handlerDeps)

	manifest, err := strategy.Build(context.Background(), params)

	s.Require().NoError(err)
	s.Nil(manifest.Lambda.SharedLayer, "no shared layer when all handlers have custom deps")

	layerA := manifest.Handlers["handlerA"].Lambda.Dependencies
	layerB := manifest.Handlers["handlerB"].Lambda.Dependencies
	s.Require().NotNil(layerA)
	s.Require().NotNil(layerB)
	// Deduplicated, so both must point to the same artifact with the same local
	// path and hash.
	s.Equal(layerA.LocalPath, layerB.LocalPath)
	s.Equal(layerA.ContentHash, layerB.ContentHash)
}

func (s *LambdaSuite) Test_Build_contentHashesAreSha256Prefixed() {
	inst := &mockInstaller{}
	reg := s.newTestRegistry(inst, nil)
	strategy := &LambdaStrategy{Registry: reg}

	appDir := s.setupAppDir()
	outputDir := s.T().TempDir()

	handlers := []HandlerInfo{
		{ResourceName: "handlerA", HandlerName: "handler.handler", CodeLocation: "handler.js"},
	}
	params := s.newBuildParams(appDir, outputDir, handlers, nil)

	manifest, err := strategy.Build(context.Background(), params)

	s.Require().NoError(err)
	s.True(strings.HasPrefix(manifest.Lambda.SharedLayer.ContentHash, "sha256:"))
	s.True(strings.HasPrefix(manifest.Lambda.AppCode.ContentHash, "sha256:"))
}

func (s *LambdaSuite) Test_Build_sizeIsRecordedOnAllArtifacts() {
	inst := &mockInstaller{}
	reg := s.newTestRegistry(inst, nil)
	strategy := &LambdaStrategy{Registry: reg}

	appDir := s.setupAppDir()
	outputDir := s.T().TempDir()

	handlers := []HandlerInfo{
		{ResourceName: "handlerA", HandlerName: "handler.handler", CodeLocation: "handler.js"},
	}
	params := s.newBuildParams(appDir, outputDir, handlers, nil)

	manifest, err := strategy.Build(context.Background(), params)

	s.Require().NoError(err)
	s.Greater(manifest.Lambda.SharedLayer.Size, int64(0))
	s.Greater(manifest.Lambda.AppCode.Size, int64(0))
}

func (s *LambdaSuite) Test_checkLayerSize_exceedsLimit_returnsError() {
	logger, _ := zap.NewDevelopment()
	layer := &Artifact{Size: LayerSizeLimit + 1}
	err := checkLayerSize(layer, "test-layer", logger)
	s.Require().Error(err)
	s.Contains(err.Error(), "250MB")
}

func (s *LambdaSuite) Test_checkLayerSize_underLimit_noError() {
	logger, _ := zap.NewDevelopment()
	layer := &Artifact{Size: LayerSizeWarning - 1}
	err := checkLayerSize(layer, "test-layer", logger)
	s.NoError(err)
}

func (s *LambdaSuite) Test_Build_withCompiler_compilesSource() {
	inst := &mockInstaller{}
	comp := &mockCompiler{}
	reg := s.newTestRegistry(inst, comp)
	strategy := &LambdaStrategy{Registry: reg}

	appDir := s.setupAppDir()
	// Also create a tsconfig.json so the real compiler would trigger,
	// but our mock just copies .js files regardless.
	outputDir := s.T().TempDir()

	handlers := []HandlerInfo{
		{ResourceName: "handlerA", HandlerName: "handler.handler", CodeLocation: "handler.js"},
	}
	params := s.newBuildParams(appDir, outputDir, handlers, nil)

	manifest, err := strategy.Build(context.Background(), params)

	s.Require().NoError(err)
	s.True(comp.compileCalled)
	s.NotNil(manifest.Lambda.AppCode)
}

// --- single shared app bundle + bootstrap behaviour ---

// stubBootstrap writes a marker file so tests can verify the bootstrap ran.
type stubBootstrap struct {
	markerName string
	entryPoint string
	generated  bool
	err        error
	// requiredPackage is empty by default so existing cases exercise bootstrap
	// generation without also asserting on the adapter dependency check.
	requiredPackage string
}

func (b *stubBootstrap) RequiredPackage() string {
	return b.requiredPackage
}

func (b *stubBootstrap) Generate(stagingDir string) (string, error) {
	b.generated = true
	if b.err != nil {
		return "", b.err
	}
	if b.markerName != "" {
		marker := filepath.Join(stagingDir, b.markerName)
		if err := os.WriteFile(marker, []byte("marker"), 0644); err != nil {
			return "", err
		}
	}
	return b.entryPoint, nil
}

func (s *LambdaSuite) newLambdaStrategyWithBootstrap(
	installer DependencyInstaller,
	bootstrap HandlerBootstrap,
) *LambdaStrategy {
	r := NewRegistry()
	r.RegisterDependencyInstaller("nodejs", installer)
	return &LambdaStrategy{
		Registry: r,
		Bootstraps: map[string]HandlerBootstrap{
			"nodejs": bootstrap,
		},
	}
}

func (s *LambdaSuite) Test_Build_producesSingleAppBundleNotPerHandlerZips() {
	inst := &mockInstaller{}
	reg := s.newTestRegistry(inst, nil)
	strategy := &LambdaStrategy{Registry: reg}

	appDir := s.setupAppDir()
	outputDir := s.T().TempDir()

	handlers := []HandlerInfo{
		{ResourceName: "handlerA", HandlerName: "handler.handler", CodeLocation: "handler.js"},
		{ResourceName: "handlerB", HandlerName: "handler.handler", CodeLocation: "handler.js"},
		{ResourceName: "handlerC", HandlerName: "handler.handler", CodeLocation: "handler.js"},
	}
	params := s.newBuildParams(appDir, outputDir, handlers, nil)

	manifest, err := strategy.Build(context.Background(), params)

	s.Require().NoError(err)
	s.Require().NotNil(manifest.Lambda.AppCode, "manifest should have a shared AppCode artifact")
	s.Len(manifest.Handlers, 3, "all handlers should still appear in the manifest")

	entries, err := os.ReadDir(outputDir)
	s.Require().NoError(err)
	appZipCount := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "app-") && strings.HasSuffix(e.Name(), ".zip") {
			appZipCount += 1
		}
		s.False(strings.HasSuffix(e.Name(), "-handler.zip"),
			"per-handler code zips should not be produced in the new shape")
	}
	s.Equal(1, appZipCount, "exactly one app-<hash>.zip should exist")
}

func (s *LambdaSuite) Test_Build_runsBootstrapAndRecordsEntryPoint() {
	inst := &mockInstaller{}
	bootstrap := &stubBootstrap{
		markerName: "__celerity_lambda_entry__.mjs",
		entryPoint: "__celerity_lambda_entry__.handler",
	}
	strategy := s.newLambdaStrategyWithBootstrap(inst, bootstrap)

	appDir := s.setupAppDir()
	outputDir := s.T().TempDir()

	handlers := []HandlerInfo{
		{ResourceName: "handlerA", HandlerName: "handler.handler", CodeLocation: "handler.js"},
	}
	params := s.newBuildParams(appDir, outputDir, handlers, nil)

	manifest, err := strategy.Build(context.Background(), params)

	s.Require().NoError(err)
	s.True(bootstrap.generated, "bootstrap.Generate should have been called")
	s.Equal("__celerity_lambda_entry__.handler", manifest.Lambda.EntryPoint)
}

func (s *LambdaSuite) Test_Build_noBootstrapRegistered_entryPointIsEmpty() {
	inst := &mockInstaller{}
	reg := s.newTestRegistry(inst, nil)
	strategy := &LambdaStrategy{Registry: reg}

	appDir := s.setupAppDir()
	outputDir := s.T().TempDir()

	handlers := []HandlerInfo{
		{ResourceName: "handlerA", HandlerName: "handler.handler", CodeLocation: "handler.js"},
	}
	params := s.newBuildParams(appDir, outputDir, handlers, nil)

	manifest, err := strategy.Build(context.Background(), params)

	s.Require().NoError(err)
	s.Equal("", manifest.Lambda.EntryPoint, "no bootstrap → empty entry point (backward compatible)")
	s.NotNil(manifest.Lambda.AppCode, "app code should still be produced")
}

func (s *LambdaSuite) Test_Build_artifactFilenamesIncludeContentHash() {
	inst := &mockInstaller{}
	reg := s.newTestRegistry(inst, nil)
	strategy := &LambdaStrategy{Registry: reg}

	appDir := s.setupAppDir()
	outputDir := s.T().TempDir()

	handlers := []HandlerInfo{
		{ResourceName: "handlerA", HandlerName: "handler.handler", CodeLocation: "handler.js"},
	}
	params := s.newBuildParams(appDir, outputDir, handlers, nil)

	manifest, err := strategy.Build(context.Background(), params)
	s.Require().NoError(err)

	appFilename := filepath.Base(manifest.Lambda.AppCode.LocalPath)
	hashPart := strings.TrimPrefix(manifest.Lambda.AppCode.ContentHash, "sha256:")
	s.Equal("app-"+hashPart+".zip", appFilename)

	s.Require().NotNil(manifest.Lambda.SharedLayer)
	layerFilename := filepath.Base(manifest.Lambda.SharedLayer.LocalPath)
	layerHash := strings.TrimPrefix(manifest.Lambda.SharedLayer.ContentHash, "sha256:")
	s.Equal("shared-layer-"+layerHash+".zip", layerFilename)
}

func (s *LambdaSuite) Test_Build_deterministic_secondBuildProducesSameHashes() {
	inst := &mockInstaller{}
	reg := s.newTestRegistry(inst, nil)
	strategy := &LambdaStrategy{Registry: reg}

	appDir := s.setupAppDir()
	outputDir := s.T().TempDir()

	handlers := []HandlerInfo{
		{ResourceName: "handlerA", HandlerName: "handler.handler", CodeLocation: "handler.js"},
	}
	params := s.newBuildParams(appDir, outputDir, handlers, nil)

	first, err := strategy.Build(context.Background(), params)
	s.Require().NoError(err)

	second, err := strategy.Build(context.Background(), params)
	s.Require().NoError(err)

	s.Equal(first.Lambda.AppCode.ContentHash, second.Lambda.AppCode.ContentHash,
		"identical source should produce the same app code hash")
}

// --- resource links bundling ---

func (s *LambdaSuite) loadBlueprint(yamlContent string) *schema.Blueprint {
	bp, err := schema.LoadString(yamlContent, schema.YAMLSpecFormat)
	s.Require().NoError(err, "failed to load test blueprint")
	return bp
}

func readFileFromZip(t interface {
	Helper()
	Fatalf(format string, args ...interface{})
}, zipPath, fileName string) []byte {
	t.Helper()
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("opening zip: %v", err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == fileName {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("opening zipped file: %v", err)
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("reading zipped file: %v", err)
			}
			return data
		}
	}
	t.Fatalf("file %q not found in zip", fileName)
	return nil
}

func (s *LambdaSuite) Test_Build_bundlesResourceLinksFromBlueprint() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersDatastore:
    type: "celerity/datastore"
    spec:
      name: users
  imagesBucket:
    type: "celerity/bucket"
    spec:
      name: images
  orderQueue:
    type: "celerity/queue"
    spec:
      name: orders
`)

	inst := &mockInstaller{}
	reg := s.newTestRegistry(inst, nil)
	strategy := &LambdaStrategy{Registry: reg}

	appDir := s.setupAppDir()
	outputDir := s.T().TempDir()

	handlers := []HandlerInfo{
		{ResourceName: "handlerA", HandlerName: "handler.handler", CodeLocation: "handler.js"},
	}
	params := s.newBuildParams(appDir, outputDir, handlers, nil)
	params.Blueprint = bp

	manifest, err := strategy.Build(context.Background(), params)
	s.Require().NoError(err)
	s.Require().NotNil(manifest.Lambda.AppCode)

	data := readFileFromZip(s.T(), manifest.Lambda.AppCode.LocalPath, ResourceLinksFilename)

	var links map[string]struct {
		Type      string `json:"type"`
		ConfigKey string `json:"configKey"`
	}
	s.Require().NoError(json.Unmarshal(data, &links))

	s.Assert().Equal("datastore", links["usersDatastore"].Type)
	s.Assert().Equal("users", links["usersDatastore"].ConfigKey)
	s.Assert().Equal("bucket", links["imagesBucket"].Type)
	s.Assert().Equal("images", links["imagesBucket"].ConfigKey)
	s.Assert().Equal("queue", links["orderQueue"].Type)
	s.Assert().Equal("orders", links["orderQueue"].ConfigKey)
}

func (s *LambdaSuite) Test_Build_bundlesEmptyResourceLinksFileWhenBlueprintNil() {
	inst := &mockInstaller{}
	reg := s.newTestRegistry(inst, nil)
	strategy := &LambdaStrategy{Registry: reg}

	appDir := s.setupAppDir()
	outputDir := s.T().TempDir()

	handlers := []HandlerInfo{
		{ResourceName: "handlerA", HandlerName: "handler.handler", CodeLocation: "handler.js"},
	}
	params := s.newBuildParams(appDir, outputDir, handlers, nil)

	manifest, err := strategy.Build(context.Background(), params)
	s.Require().NoError(err)

	data := readFileFromZip(s.T(), manifest.Lambda.AppCode.LocalPath, ResourceLinksFilename)
	s.Assert().JSONEq("{}", string(data))
}

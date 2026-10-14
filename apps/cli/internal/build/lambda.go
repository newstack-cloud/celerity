package build

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/celerity/apps/cli/internal/seed"
	"go.uber.org/zap"
)

// LambdaStrategy packages a Celerity application for AWS Lambda as a
// single shared app code bundle (referenced by every handler's Lambda
// function) plus a shared or per-handler dependency layer. Per-handler
// dep layers are opt-in via handlerDependencies declarations in the
// developer's package manifest.
type LambdaStrategy struct {
	Registry *Registry
	// Bootstraps maps runtime prefixes (e.g. "nodejs", "python") to the
	// Lambda-specific handler bootstrap for that language. Kept on the
	// strategy rather than the shared registry because bootstraps are
	// deploy-target-specific, and a future GCP strategy would ship its own
	// mapping with different SDK imports.
	Bootstraps map[string]HandlerBootstrap
}

// Matched by prefix so a versioned identifier like "nodejs24.x" resolves to
// the "nodejs" entry. Returns nil for a runtime with no registered bootstrap,
// which leaves the build working for runtimes without SDK wrapping.
func (s *LambdaStrategy) bootstrapFor(runtime string) HandlerBootstrap {
	for prefix, bootstrap := range s.Bootstraps {
		if strings.HasPrefix(runtime, prefix) {
			return bootstrap
		}
	}
	return nil
}

func (s *LambdaStrategy) Build(ctx context.Context, params *BuildParams) (*Manifest, error) {
	if err := os.MkdirAll(params.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	// Checked before the dependency installer is looked up, which errors for a
	// runtime nothing is registered for. A compiled language needs no installer
	// because its dependencies are linked into the executable.
	if params.Toolchain != nil {
		return s.buildCompiledManifest(ctx, params)
	}

	installer, err := s.Registry.GetDependencyInstaller(params.Runtime)
	if err != nil {
		return nil, err
	}
	compiler := s.Registry.GetSourceCompiler(params.Runtime)

	compiledDir, err := compileSource(ctx, params, compiler)
	if err != nil {
		return nil, err
	}
	if compiledDir != "" {
		defer os.RemoveAll(compiledDir)
	}

	manifest := &Manifest{
		Version:  ManifestVersion,
		Runtime:  params.Runtime,
		Target:   params.DeployTarget,
		Handlers: map[string]*HandlerArtifacts{},
		Lambda:   &LambdaManifest{},
	}

	params.Logger.Info("building shared app code bundle")
	appCode, entryPoint, err := s.buildAppArchive(params, compiledDir)
	if err != nil {
		return nil, fmt.Errorf("building app code bundle: %w", err)
	}
	manifest.Lambda.AppCode = appCode
	manifest.Lambda.EntryPoint = entryPoint

	if err := s.buildDependencyLayers(ctx, installer, params, manifest); err != nil {
		return nil, err
	}

	runner, err := s.buildMaintenanceRunner(params)
	if err != nil {
		return nil, fmt.Errorf("building the database maintenance runner: %w", err)
	}
	manifest.Lambda.MaintenanceRunner = runner

	if params.Uploader != nil {
		if err := uploadAllArtifacts(ctx, params, manifest); err != nil {
			return nil, fmt.Errorf("uploading artifacts: %w", err)
		}
	}

	return manifest, nil
}

func (s *LambdaStrategy) buildDependencyLayers(
	ctx context.Context,
	installer DependencyInstaller,
	params *BuildParams,
	manifest *Manifest,
) error {
	sharedHandlers, customHandlers := partitionHandlers(params.Handlers, params.HandlerDependencies)

	if len(sharedHandlers) > 0 {
		params.Logger.Info("building shared dependency layer")
		layer, err := buildLayer(ctx, installer, params, nil, "shared-layer")
		if err != nil {
			return fmt.Errorf("building shared layer: %w", err)
		}
		if err := checkLayerSize(layer, "shared", params.Logger); err != nil {
			return err
		}
		manifest.Lambda.SharedLayer = layer
	}

	customLayers, err := buildCustomLayers(ctx, installer, params, customHandlers)
	if err != nil {
		return err
	}

	for _, handler := range params.Handlers {
		arts := &HandlerArtifacts{Lambda: &LambdaHandlerArtifacts{}}
		if layer, ok := customLayers[handler.ResourceName]; ok {
			arts.Lambda.Dependencies = layer
		}
		manifest.Handlers[handler.ResourceName] = arts
	}
	return nil
}

// The bundle is the whole source tree, compiled if applicable, plus the
// per-runtime Celerity SDK bootstrap file. The entry point comes back empty for
// a runtime with no registered bootstrap.
func (s *LambdaStrategy) buildAppArchive(
	params *BuildParams,
	compiledDir string,
) (*Artifact, string, error) {
	stagingDir, err := os.MkdirTemp("", "celerity-app-*")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(stagingDir)

	sourceDir := params.AppDir
	if compiledDir != "" {
		sourceDir = compiledDir
	}
	if err := copySourceForLambda(sourceDir, stagingDir); err != nil {
		return nil, "", err
	}

	if err := writeBundleModuleManifest(params.Runtime, params.AppDir, stagingDir); err != nil {
		return nil, "", err
	}

	entryPoint, err := s.writeHandlerBootstrap(params.Runtime, params.AppDir, stagingDir)
	if err != nil {
		return nil, "", err
	}

	if err := writeResourceLinksFile(stagingDir, params.Blueprint); err != nil {
		return nil, "", err
	}

	art, err := writeArtifact(ArtifactTypeLambdaArchive, stagingDir, params.OutputDir, "app")
	if err != nil {
		return nil, "", err
	}
	return art, entryPoint, nil
}

// The blueprint-derived links JSON rides in the Lambda app archive so the SDK
// can read it on cold start rather than through a large env var. It is written
// even when the blueprint has no qualifying resources, which keeps the SDK
// contract to a file that is always present.
func writeResourceLinksFile(stagingDir string, bp *schema.Blueprint) error {
	linksJSON, err := seed.ResourceLinksJSON(bp)
	if err != nil {
		return fmt.Errorf("generating resource links JSON: %w", err)
	}
	if linksJSON == "" {
		linksJSON = "{}"
	}
	target := filepath.Join(stagingDir, ResourceLinksFilename)
	if err := os.WriteFile(target, []byte(linksJSON), 0644); err != nil {
		return fmt.Errorf("writing resource links file: %w", err)
	}

	return writeHandlerScopeFile(stagingDir, bp)
}

// The per-handler resource scope rides alongside the resource links so a
// handler initialises only the resources it uses rather than every resource the
// application declares.
//
// Written unconditionally, like the links file, so the SDK always has a file to
// parse. An empty payload means no handler declared a scope, which the SDK
// treats as absent scope information and falls back from, rather than as no
// handler using anything.
func writeHandlerScopeFile(stagingDir string, bp *schema.Blueprint) error {
	scopeJSON, err := seed.HandlerScopeJSON(bp)
	if err != nil {
		return fmt.Errorf("generating handler scope JSON: %w", err)
	}
	if scopeJSON == "" {
		scopeJSON = "{}"
	}
	target := filepath.Join(stagingDir, HandlerScopeFilename)
	if err := os.WriteFile(target, []byte(scopeJSON), 0644); err != nil {
		return fmt.Errorf("writing handler scope file: %w", err)
	}
	return nil
}

func (s *LambdaStrategy) writeHandlerBootstrap(
	runtime string,
	appDir string,
	stagingDir string,
) (string, error) {
	bootstrap := s.bootstrapFor(runtime)
	if bootstrap == nil {
		return "", nil
	}

	// Checked before the file is written rather than after the deploy. The entry
	// file imports this package, and a bundle without it produces handlers that
	// provision cleanly and then fail on their first invocation with a module
	// resolution error, a long way from the missing dependency that caused it.
	if err := requireAdapterPackage(bootstrap, runtime, appDir); err != nil {
		return "", err
	}

	entryPoint, err := bootstrap.Generate(stagingDir)
	if err != nil {
		return "", fmt.Errorf("generating Lambda bootstrap: %w", err)
	}
	return entryPoint, nil
}

// A check for "some Celerity SDK package" is not enough, because an app can
// depend on several of them and still be missing the serverless adapter. That
// is how a project reached a real deployment with every handler unable to
// start.
func requireAdapterPackage(bootstrap HandlerBootstrap, runtime, appDir string) error {
	required := bootstrap.RequiredPackage()
	if required == "" || appDir == "" {
		return nil
	}

	declared, err := appDeclaresPackage(runtime, appDir, required)
	if err != nil {
		// An unreadable manifest is not proof the dependency is absent, and
		// failing the build on it would be worse than the check not running.
		return nil
	}
	if declared {
		return nil
	}

	return fmt.Errorf(
		"the generated Lambda entry file imports %q, but it is not a dependency of the "+
			"application; add %q to the project's dependencies so it is included in the "+
			"deployment bundle",
		required, required,
	)
}

func partitionHandlers(
	handlers []HandlerInfo,
	handlerDeps map[string][]string,
) (shared, custom []HandlerInfo) {
	if len(handlerDeps) == 0 {
		return handlers, nil
	}
	for _, h := range handlers {
		if _, ok := handlerDeps[h.ResourceName]; ok {
			custom = append(custom, h)
		} else {
			shared = append(shared, h)
		}
	}
	return shared, custom
}

func buildCustomLayers(
	ctx context.Context,
	installer DependencyInstaller,
	params *BuildParams,
	handlers []HandlerInfo,
) (map[string]*Artifact, error) {
	if len(handlers) == 0 {
		return nil, nil
	}

	layers := map[string]*Artifact{}
	// Deduplicate identical dep sets.
	type dedupEntry struct {
		key      string
		packages []string
		artifact *Artifact
	}
	seen := map[string]*dedupEntry{}

	for _, h := range handlers {
		packages := params.HandlerDependencies[h.ResourceName]
		key := depSetKey(packages)
		if entry, exists := seen[key]; exists {
			layers[h.ResourceName] = entry.artifact
			continue
		}

		params.Logger.Info("building handler layer",
			zap.String("handler", h.ResourceName),
			zap.Int("packages", len(packages)),
		)
		prefix := h.ResourceName + "-layer"
		layer, err := buildLayer(ctx, installer, params, packages, prefix)
		if err != nil {
			return nil, fmt.Errorf("building layer for %s: %w", h.ResourceName, err)
		}
		if err := checkLayerSize(layer, h.ResourceName, params.Logger); err != nil {
			return nil, err
		}

		seen[key] = &dedupEntry{key: key, packages: packages, artifact: layer}
		layers[h.ResourceName] = layer
	}

	return layers, nil
}

func buildLayer(
	ctx context.Context,
	installer DependencyInstaller,
	params *BuildParams,
	packages []string,
	prefix string,
) (*Artifact, error) {
	stagingDir, err := os.MkdirTemp("", "celerity-layer-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stagingDir)

	installDir := filepath.Join(stagingDir, lambdaLayerSubdir(params.Runtime))
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return nil, err
	}

	if err := installer.InstallDependencies(ctx, params.AppDir, installDir, packages, params.ToolOutput); err != nil {
		return nil, err
	}

	return writeArtifact(ArtifactTypeLambdaLayer, stagingDir, params.OutputDir, prefix)
}

func compileSource(
	ctx context.Context,
	params *BuildParams,
	compiler SourceCompiler,
) (string, error) {
	if compiler == nil {
		return "", nil
	}

	params.Logger.Info("compiling source", zap.String("runtime", params.Runtime))
	outDir, err := os.MkdirTemp("", "celerity-compiled-*")
	if err != nil {
		return "", err
	}

	if err := compiler.Compile(ctx, params.AppDir, outDir, params.ToolOutput); err != nil {
		os.RemoveAll(outDir)
		return "", fmt.Errorf("compiling source: %w", err)
	}

	return outDir, nil
}

func uploadAllArtifacts(ctx context.Context, params *BuildParams, manifest *Manifest) error {
	if manifest.Lambda == nil {
		return nil
	}
	prefix := params.InstanceName
	if prefix == "" {
		prefix = "default"
	}

	if manifest.Lambda.AppCode != nil {
		key := fmt.Sprintf("%s/app/%s.zip", prefix, trimHashPrefix(manifest.Lambda.AppCode.ContentHash))
		if err := params.Uploader.Upload(ctx, manifest.Lambda.AppCode, key); err != nil {
			return fmt.Errorf("uploading app code: %w", err)
		}
	}

	if manifest.Lambda.SharedLayer != nil {
		key := fmt.Sprintf("%s/shared-layer/%s.zip", prefix, trimHashPrefix(manifest.Lambda.SharedLayer.ContentHash))
		if err := params.Uploader.Upload(ctx, manifest.Lambda.SharedLayer, key); err != nil {
			return fmt.Errorf("uploading shared layer: %w", err)
		}
	}

	if manifest.Lambda.MaintenanceRunner != nil {
		key := fmt.Sprintf("%s/maintenance-runner/%s.zip",
			prefix, trimHashPrefix(manifest.Lambda.MaintenanceRunner.ContentHash))
		if err := params.Uploader.Upload(ctx, manifest.Lambda.MaintenanceRunner, key); err != nil {
			return fmt.Errorf("uploading the maintenance runner: %w", err)
		}
	}

	for name, arts := range manifest.Handlers {
		if arts.Lambda == nil || arts.Lambda.Dependencies == nil {
			continue
		}
		key := fmt.Sprintf("%s/%s/layer/%s.zip", prefix, name, trimHashPrefix(arts.Lambda.Dependencies.ContentHash))
		if err := params.Uploader.Upload(ctx, arts.Lambda.Dependencies, key); err != nil {
			return fmt.Errorf("uploading handler %s layer: %w", name, err)
		}
	}

	return nil
}

// Writes to a temp name, hashes, then renames to "<prefix>-<hash>.zip", so the
// on-disk filename always matches the content and a stale build cannot hide
// behind a stable name.
func writeArtifact(
	artifactType string,
	stagingDir string,
	outputDir string,
	prefix string,
) (*Artifact, error) {
	tempPath := filepath.Join(outputDir, prefix+"-tmp.zip")
	if err := createZipArchive(stagingDir, tempPath); err != nil {
		return nil, err
	}

	hash, err := fileContentHash(tempPath)
	if err != nil {
		os.Remove(tempPath)
		return nil, fmt.Errorf("hashing artifact: %w", err)
	}

	finalPath := filepath.Join(outputDir, fmt.Sprintf("%s-%s.zip", prefix, hash))
	if err := os.Rename(tempPath, finalPath); err != nil {
		return nil, fmt.Errorf("renaming artifact: %w", err)
	}

	info, err := os.Stat(finalPath)
	if err != nil {
		return nil, fmt.Errorf("stat artifact: %w", err)
	}

	return &Artifact{
		Type:        artifactType,
		LocalPath:   finalPath,
		ContentHash: "sha256:" + hash,
		Size:        info.Size(),
	}, nil
}

func checkLayerSize(layer *Artifact, name string, logger *zap.Logger) error {
	if layer.Size > LayerSizeLimit {
		return fmt.Errorf(
			"layer %q is %d bytes (%d MB), exceeding the Lambda 250MB limit",
			name, layer.Size, layer.Size/(1024*1024),
		)
	}
	if layer.Size > LayerSizeWarning {
		logger.Warn("layer approaching Lambda size limit",
			zap.String("layer", name),
			zap.Int64("sizeBytes", layer.Size),
			zap.Int64("limitBytes", LayerSizeLimit),
		)
	}
	return nil
}

func depSetKey(packages []string) string {
	sorted := make([]string, len(packages))
	copy(sorted, packages)
	sort.Strings(sorted)
	return strings.Join(sorted, "\n")
}

func trimHashPrefix(hash string) string {
	return strings.TrimPrefix(hash, "sha256:")
}

// The directory within the layer archive that dependencies are installed into.
//
// Node stops at "nodejs" because npm creates node_modules inside whatever
// directory it installs into, so naming node_modules here put every package at
// /opt/nodejs/node_modules/node_modules/<pkg>, one level below where the runtime
// looks, and nothing in the layer resolved. Python keeps the full path because
// pip installs into the directory it is given rather than creating one.
func lambdaLayerSubdir(runtime string) string {
	switch {
	case strings.HasPrefix(runtime, "nodejs"):
		return "nodejs"
	case strings.HasPrefix(runtime, "python"):
		version := strings.TrimPrefix(runtime, "python")
		return fmt.Sprintf("python/lib/python%s/site-packages", version)
	default:
		return "lib"
	}
}

func copySourceForLambda(appDir, stagingDir string) error {
	return filepath.WalkDir(appDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(appDir, path)
		if err != nil {
			return err
		}

		if shouldSkipForLambda(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		dest := filepath.Join(stagingDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}

		return copyFile(path, dest)
	})
}

var lambdaSkipDirs = map[string]bool{
	"node_modules": true, ".git": true, ".celerity": true,
	"__pycache__": true, ".pytest_cache": true, ".mypy_cache": true,
	".venv": true, "venv": true,
	"test": true, "tests": true, "__tests__": true,
	"coverage": true, ".nyc_output": true,
}

func shouldSkipForLambda(rel string, d fs.DirEntry) bool {
	if d.IsDir() && lambdaSkipDirs[d.Name()] {
		return true
	}
	if strings.HasPrefix(rel, "app.blueprint") || strings.HasPrefix(rel, "app.deploy") {
		return true
	}
	return false
}

// Stamped on every archive entry so builds are byte-for-byte reproducible.
var reproducibleArchiveTime = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

func createZipArchive(sourceDir, outputPath string) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	w := zip.NewWriter(outFile)
	defer w.Close()

	return filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		header.Method = zip.Deflate

		// A fixed timestamp keeps the same inputs producing the same archive.
		// Modification times would otherwise feed the content hash, and a
		// dependency layer is rebuilt by re-running the package manager, which
		// rewrites every mtime. That yields a new hash, a new layer resource
		// name, and a deploy whose blueprint no longer matches the change set
		// staged from the previous build. The date is arbitrary and only has to
		// be stable and after 1980, the earliest a zip entry can express.
		header.Modified = reproducibleArchiveTime

		if info.Mode()&0111 != 0 {
			header.SetMode(info.Mode())
		}

		writer, err := w.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func fileContentHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Declares the bundle's module system to the Node runtime.
//
// Compiling replaces the copied source tree with the compiler's output
// directory, which holds emitted JavaScript and nothing else, so the project's
// package.json never reached the archive. Without one, Node reads every .js
// file as CommonJS, and importing the app's own module from the generated .mjs
// entry fails with "Cannot use import statement outside a module".
//
// Only the module type is written, not a copy of the project manifest. A
// manifest carrying dependencies, main and exports describes a package that is
// installed, and this bundle is unpacked rather than installed.
const bundleModuleManifest = `{"type": "module"}` + "\n"

func writeBundleModuleManifest(runtime, appDir, stagingDir string) error {
	if !strings.HasPrefix(runtime, "nodejs") || !isESMProject(appDir) {
		return nil
	}

	target := filepath.Join(stagingDir, "package.json")
	// A manifest already in the bundle came from the project's own source tree,
	// where the author decides what it says.
	if _, err := os.Stat(target); err == nil {
		return nil
	}

	if err := os.WriteFile(target, []byte(bundleModuleManifest), 0644); err != nil {
		return fmt.Errorf("writing the bundle module manifest: %w", err)
	}
	return nil
}

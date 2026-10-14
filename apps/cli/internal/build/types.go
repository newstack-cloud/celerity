// Package build provides application build and packaging logic for
// Celerity application deployments.
package build

import (
	"context"
	"io"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/celerity/apps/cli/buildmanifest"
	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
	"go.uber.org/zap"
)

// The build manifest wire types are defined once in the public buildmanifest
// package (the single source of truth shared with the Celerity transformer)
// and re-exported here as aliases so existing internal/build call sites are
// unchanged.
type (
	Manifest               = buildmanifest.Manifest
	HandlerArtifacts       = buildmanifest.HandlerArtifacts
	Artifact               = buildmanifest.Artifact
	LambdaManifest         = buildmanifest.LambdaManifest
	LambdaHandlerArtifacts = buildmanifest.LambdaHandlerArtifacts
)

const (
	ManifestVersion = buildmanifest.Version
	ManifestFile    = buildmanifest.FileName

	ArtifactTypeContainerImage = buildmanifest.ArtifactTypeContainerImage

	// ResourceLinksFilename is the name of the blueprint-derived resource
	// links JSON file bundled into the Lambda app archive. The SDK reads
	// it from the handler's module directory on cold start, removing the
	// need to pass the payload through environment variables (which have
	// a tight 4 KB total limit on AWS Lambda).
	ResourceLinksFilename = buildmanifest.ResourceLinksFilename

	// HandlerScopeFilename is the per-handler resource scope file bundled
	// alongside the resource links.
	HandlerScopeFilename = buildmanifest.HandlerScopeFilename
)

// HandlerInfo describes a handler to be built.
type HandlerInfo struct {
	ResourceName string
	HandlerName  string
	CodeLocation string
}

// BuildParams holds the inputs needed to execute a build.
type BuildParams struct {
	AppDir              string
	Runtime             string
	DeployTarget        string
	Handlers            []HandlerInfo
	HandlerDependencies map[string][]string
	OutputDir           string
	Uploader            ArtifactUploader
	InstanceName        string
	Logger              *zap.Logger
	// Blueprint is the merged blueprint used to derive build-time artifacts
	// like the resource links file bundled into the Lambda app archive.
	Blueprint *schema.Blueprint
	// ToolOutput is the writer for tool output (npm, pip, tsc).
	// In interactive mode this should be a log file; in headless/standalone
	// mode it should be os.Stdout.
	ToolOutput io.Writer
	// Toolchain builds the handlers executable for an OS-only runtime, and is
	// nil for a runtime that hosts the language itself. Its presence is what
	// selects the compiled packaging path.
	Toolchain handlertoolchain.Toolchain
}

// Strategy determines how an application is packaged for a given deploy target.
type Strategy interface {
	Build(ctx context.Context, params *BuildParams) (*Manifest, error)
}

// DependencyInstaller installs dependencies into a target directory.
// When packages is nil, all production dependencies are installed.
// When packages is non-nil, only those packages are installed.
// toolOutput receives tool stdout/stderr (npm, pip output).
type DependencyInstaller interface {
	InstallDependencies(ctx context.Context, appDir, targetDir string, packages []string, toolOutput io.Writer) error
}

// SourceCompiler compiles or transpiles source code for deployment,
// preserving the directory structure for readability.
// toolOutput receives compiler stdout/stderr (tsc output).
type SourceCompiler interface {
	Compile(ctx context.Context, srcDir, outDir string, toolOutput io.Writer) error
}

// CommandExecutor runs external commands. The default implementation
// uses os/exec; tests can substitute a stub.
type CommandExecutor interface {
	Run(ctx context.Context, dir string, toolOutput io.Writer, name string, args ...string) error
}

// ArtifactUploader uploads build artifacts to remote storage.
type ArtifactUploader interface {
	Upload(ctx context.Context, artifact *Artifact, key string) error
	UploadManifest(ctx context.Context, data []byte, instanceName string) (string, error)
}

// HandlerDependencyReader reads per-handler dependency declarations
// from a language-specific manifest file.
type HandlerDependencyReader interface {
	ReadHandlerDependencies(appDir string) (map[string][]string, error)
}

// HandlerBootstrap writes a per-language entry point file that wraps the
// developer's app in the appropriate Celerity SDK serverless adapter for
// the deploy target. Returns the entry point string that the strategy
// should record on its target-specific sub-manifest.
//
// Generate is called once per build rather than once per handler, since the
// bootstrap is identical across all handlers in a project and the shared app
// code bundle contains a single copy of it.
//
// Implementations are strategy-specific (e.g. LambdaNodeBootstrap wraps
// the app for AWS Lambda). A future GCP strategy would ship its own
// implementations targeting the GCP serverless adapter.
type HandlerBootstrap interface {
	Generate(stagingDir string) (entryPoint string, err error)

	// RequiredPackage names the adapter package the generated entry file
	// imports, so the build can check the app actually depends on it.
	//
	// Declared here rather than checked elsewhere because the import and the
	// requirement have to move together. A bootstrap whose import changed
	// while a check somewhere else kept naming the old package would produce
	// handlers that provision cleanly and then fail on their first
	// invocation, a long way from the mistake.
	RequiredPackage() string
}

// Package handlertoolchain resolves and drives the build tooling for an
// OS-only runtime.
//
// An OS-only runtime identifier (os-only.debian13) names a base operating
// system rather than a language, so unlike "nodejs24.x" it does not give the
// language away and the toolchain has to be resolved from the project instead.
// Two implementations ship, one driving the Celerity Go SDK's celerity-go tool
// and a declarative one driven by a handlerBuild block in the application's
// deploy config, which is how a community SDK for another compiled language is
// supported without CLI code of its own.
//
// This package is deliberately a leaf. internal/deployconfig already imports
// internal/build and internal/handlerresolve, so a toolchain importing
// deployconfig back would close a cycle. The handlerBuild schema therefore
// lives here and deploy config embeds it.
package handlertoolchain

import (
	"context"

	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
)

// Build modes. A dev session reads its own local config store and talks to
// local emulators, so it links different packages from a deployed artefact.
const (
	ModeDev    = "dev"
	ModeDeploy = "deploy"
)

// TelemetryOTel is what PrepareParams.Telemetry carries to link an
// OpenTelemetry exporter. Its own setting rather than one derived from the
// deploy target, since an application deployed anywhere may export anywhere.
const TelemetryOTel = "otel"

// PrepareParams describes the code generation a toolchain does before either
// building or extracting.
type PrepareParams struct {
	AppDir string
	// DeployTarget is the app's deploy target (e.g. "aws-serverless"), which
	// decides the platform packages linked into the executable.
	DeployTarget string
	// Mode is ModeDev or ModeDeploy.
	Mode string
	// ResourceKinds are the Celerity resource kinds the blueprint declares
	// (bucket, queue, topic, cache, datastore, sqlDatabase), so the toolchain
	// links an implementation for each.
	ResourceKinds []string
	// SQLEngines are the engines of the blueprint's SQL databases
	// (e.g. "postgres"), so a driver is linked for each.
	SQLEngines []string
	// Telemetry names what traces are exported with, or is empty for none.
	Telemetry string
	// TestPackages links the platform packages into the project's integration
	// suites as well as into the application, so a test that reaches a real
	// resource reaches it through the same provider the deployed binary would.
	// Set for a dev session and not for a deploy, where there are no tests to
	// generate for.
	TestPackages bool
}

// ExtractParams describes a handler extraction run.
type ExtractParams struct {
	AppDir string
}

// BinaryParams describes one handlers executable to produce.
type BinaryParams struct {
	AppDir string
	// OS and Arch are the platform the executable has to run on rather than
	// the one building it. A dev session targets the Docker daemon's
	// architecture, and a Lambda build targets the architecture the function
	// is declared with.
	OS   string
	Arch string
	// OutputPath is where the executable must end up.
	OutputPath string
	// DeployTarget and Mode mirror PrepareParams.
	DeployTarget string
	Mode         string
}

// WatchPaths describes what a dev session's file watcher should follow.
//
// Returned by the toolchain rather than read from the runtime conventions
// because all three are properties of the language. A Go module keeps its
// handlers at the module root while a Node project keeps them under src/, and
// a Rust project needs the .toml files no other language does.
type WatchPaths struct {
	// Roots are directories relative to the application directory.
	Roots []string
	// Extensions are the file extensions a change is relevant for.
	Extensions []string
	// SkipDirs are directory names not to descend into.
	SkipDirs []string
	// SkipFiles are base names a change is never relevant for, whatever their
	// extension.
	//
	// A toolchain's own generated file has to be listed here or a session
	// cannot settle, because Prepare rewrites that file and reacting to the
	// rewrite runs Prepare again.
	SkipFiles []string
}

// Toolchain turns an application's source into a handlers executable for an
// OS-only runtime, and reports what that executable serves.
type Toolchain interface {
	// Name identifies the toolchain in output and errors (e.g. "go").
	Name() string

	// Validate reports whether the project is actually set up for this
	// toolchain, so a missing SDK dependency fails before a build does.
	Validate(appDir string) error

	// Prepare runs whatever code generation has to happen before the
	// application will compile for this target. It is safe to call more than
	// once and regenerates rather than reusing what is on disk, because a dev
	// session's generated file is the wrong one for a deploy and vice versa.
	Prepare(ctx context.Context, p PrepareParams) error

	// Extract reports the handlers the executable serves. Returning a nil
	// manifest means this toolchain cannot extract, and the blueprint's
	// hand-authored handler resources are used as they are.
	//
	// Callers run Prepare first: extraction compiles the application, so it
	// fails on a project whose generated platform imports are absent or stale.
	Extract(ctx context.Context, p ExtractParams) (*preprocess.HandlerManifest, error)

	// BuildBinary compiles the handlers executable to p.OutputPath.
	BuildBinary(ctx context.Context, p BinaryParams) error

	// WatchPaths describes what a dev session should watch.
	WatchPaths() WatchPaths

	// TestCommand returns the command for a named test suite, and whether one
	// is configured.
	TestCommand(suite string) ([]string, bool)
}

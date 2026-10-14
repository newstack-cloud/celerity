package handlertoolchain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
)

const (
	// GoToolBinary is the Celerity Go SDK's build-time tool.
	GoToolBinary = "celerity-go"

	// GoToolModule is the module that tool is in. Its own module rather than
	// the SDK's, so an application links no part of the build tooling.
	GoToolModule = GoSDKModule + "/cmd/" + GoToolBinary

	// GoToolEnvVar overrides where that tool is found, for a checkout that
	// has not installed it on PATH.
	GoToolEnvVar = "CELERITY_GO_TOOL"

	// goGeneratedFile is what celerity-go generate writes. Target-specific,
	// and expected to be gitignored.
	goGeneratedFile = "celerity_platform_gen.go"
)

// goToolchain drives the Celerity Go SDK's celerity-go tool.
type goToolchain struct {
	runner Runner
}

func newGoToolchain(runner Runner) *goToolchain { return &goToolchain{runner: runner} }

func (t *goToolchain) Name() string { return "go" }

func (t *goToolchain) Validate(appDir string) error {
	declared, err := declaresGoSDK(appDir)
	if err != nil {
		return err
	}
	if !declared {
		return fmt.Errorf(
			"this is a Go application on an OS-only runtime, but go.mod does not require %s.\n"+
				"Run: go get %s",
			GoSDKModule, GoSDKModule,
		)
	}
	return nil
}

// Prepare writes the platform imports for the deploy target and reconciles
// go.mod.
//
// Regenerated every time rather than reused, because what generate writes is
// chosen for one deploy target and one mode. The file a dev session leaves
// behind links the local config store and emulator clients that a deployed
// artefact must not carry.
func (t *goToolchain) Prepare(ctx context.Context, p PrepareParams) error {
	tool, err := goToolCommand(p.AppDir)
	if err != nil {
		return err
	}

	args := []string{"generate", "--target", p.DeployTarget, "--package", "."}
	if p.Mode == ModeDev {
		args = append(args, "--local")
	}
	if len(p.ResourceKinds) > 0 {
		args = append(args, "--resource", strings.Join(p.ResourceKinds, ","))
	}
	if len(p.SQLEngines) > 0 {
		args = append(args, "--sql-engine", strings.Join(p.SQLEngines, ","))
	}
	if p.Telemetry != "" {
		args = append(args, "--telemetry", p.Telemetry)
	}
	if p.TestPackages {
		// No --test-tags alongside it. The tool defaults to the integration
		// tag, which is the one TestCommand runs the integration suite under,
		// so naming it here would be a second place for the two to disagree.
		args = append(args, "--test-packages")
	}

	name, args := invocation(tool, args...)
	if err := t.runner.Run(ctx, p.AppDir, nil, toolOutput(ctx), name, args...); err != nil {
		return fmt.Errorf("%s generate failed: %w", GoToolBinary, err)
	}

	// generate writes imports go.mod does not declare, so the module has to be
	// reconciled before anything will compile.
	//
	// tidy rather than `go mod download`, which resolves nothing that is not
	// required yet, or `go get` of each platform module, which would need a
	// second list of what generate writes kept in step here. The cost is
	// tidy's breadth, being a whole-module reconciliation that also drops a
	// requirement nothing imports any more.
	if err := t.runner.Run(ctx, p.AppDir, nil, toolOutput(ctx), "go", "mod", "tidy"); err != nil {
		return fmt.Errorf(
			"go mod tidy failed after %s generate: %w.\n"+
				"generate writes platform imports that go.mod does not yet declare, so the module "+
				"has to be reconciled before the application will build",
			GoToolBinary, err,
		)
	}
	return nil
}

func (t *goToolchain) Extract(ctx context.Context, p ExtractParams) (*preprocess.HandlerManifest, error) {
	tool, err := goToolCommand(p.AppDir)
	if err != nil {
		return nil, err
	}

	// celerity-go writes the manifest to a file and prints a human summary to
	// standard output, so unlike the Node and Python extractors there is
	// nothing to read off the pipe.
	out, err := os.CreateTemp("", "celerity-handler-manifest-*.json")
	if err != nil {
		return nil, fmt.Errorf("making somewhere for the handler manifest: %w", err)
	}
	manifestPath := out.Name()
	_ = out.Close()
	defer os.Remove(manifestPath)

	name, args := invocation(tool, "extract", "--package", ".", "--out", manifestPath)
	err = t.runner.Run(ctx, p.AppDir, nil, toolOutput(ctx), name, args...)
	if err != nil {
		return nil, fmt.Errorf("%s extract failed: %w", GoToolBinary, err)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading the handler manifest %s wrote: %w", GoToolBinary, err)
	}

	var manifest preprocess.HandlerManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing the handler manifest: %w", err)
	}
	if err := preprocess.ValidateManifest(&manifest); err != nil {
		return nil, fmt.Errorf("%s produced an unusable manifest: %w", GoToolBinary, err)
	}
	return &manifest, nil
}

func (t *goToolchain) BuildBinary(ctx context.Context, p BinaryParams) error {
	output, err := filepath.Abs(p.OutputPath)
	if err != nil {
		return fmt.Errorf("resolving the handlers executable output path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("creating the output directory: %w", err)
	}

	// Statically linked, because the image the executable is built against is
	// not the one it ends up on. An OS-only runtime identifier maps to a
	// different base OS on each deploy target, and a dynamically linked binary
	// built on the newest of them will not start on the oldest.
	env := map[string]string{
		"CGO_ENABLED": "0",
		"GOOS":        p.OS,
		"GOARCH":      p.Arch,
	}
	args := []string{"build", "-trimpath", "-ldflags", "-s -w", "-o", output, "."}
	if err := t.runner.Run(ctx, p.AppDir, env, toolOutput(ctx), "go", args...); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	if err := ensureExecutable(output); err != nil {
		return err
	}
	return CheckArchitecture(output, p.Arch)
}

func (t *goToolchain) WatchPaths() WatchPaths {
	return WatchPaths{
		// The module root rather than src/, since a Go module keeps its
		// handlers wherever its packages are and the main package is commonly
		// under cmd/.
		Roots: []string{"."},
		// Source only. go.mod is rewritten by the `go mod tidy` that Prepare
		// runs, so watching it would put the same rebuild loop back by
		// another route.
		Extensions: []string{".go"},
		SkipDirs:   []string{".celerity", "vendor"},
		// Rewritten by every Prepare, which a rebuild runs, so reacting to it
		// would make each rebuild trigger the next.
		SkipFiles: []string{goGeneratedFile},
	}
}

func (t *goToolchain) TestCommand(suite string) ([]string, bool) {
	switch suite {
	case "unit":
		return []string{"go", "test", "./..."}, true
	case "integration":
		return []string{"go", "test", "-tags", "integration", "./..."}, true
	case "api":
		return []string{"go", "test", "-tags", "api", "./..."}, true
	default:
		return nil, false
	}
}

// The prefix is more than one word where celerity-go is reached through the Go
// toolchain rather than as a binary of its own, so the subcommand cannot
// simply be the first argument.
func invocation(prefix []string, args ...string) (string, []string) {
	full := make([]string, 0, len(prefix)-1+len(args))
	full = append(full, prefix[1:]...)
	full = append(full, args...)
	return prefix[0], full
}

// Returns the command that runs celerity-go, as a prefix the subcommand is
// appended to.
//
// The tool dependency is preferred over PATH, and that order is the point.
// celerity-go generate writes imports of SDK packages and extract emits the
// SDK's manifest format, so the tool and the SDK the application compiles
// against have to be the same version. A tool dependency is resolved by the
// same minimal version selection as the SDK requirement beside it, making
// agreement structural, while one on PATH is whatever was installed globally
// and last.
//
// Nothing is downloaded, and `go run <module>@<version>` is not used either.
// The tool shells out to go build, go mod tidy and go list, so it cannot run
// without a Go toolchain and a fetched binary would save nothing, and a
// version guessed here would be the very mismatch the ordering above prevents.
func goToolCommand(appDir string) ([]string, error) {
	if override := os.Getenv(GoToolEnvVar); override != "" {
		return []string{override}, nil
	}

	goMod, err := readGoMod(appDir)
	if err != nil {
		return nil, err
	}
	if declaresTool(goMod, GoToolModule) {
		return []string{"go", "tool", GoToolBinary}, nil
	}

	if path, err := exec.LookPath(GoToolBinary); err == nil {
		return []string{path}, nil
	}

	return nil, fmt.Errorf("%s", missingGoTool(goMod))
}

// The fix is named at the version the application already requires rather
// than at whatever is newest, since the two have to match.
func missingGoTool(goMod string) string {
	at := ""
	if version, found := requires(goMod, GoSDKModule); found && version != "" {
		at = "@" + version
	}

	return fmt.Sprintf(
		"%s is not available, and it is what builds a Go application for a deploy target.\n"+
			"Declare it alongside the SDK so the two stay at one version:\n"+
			"  go get -tool %s%s\n"+
			"Alternatively put it on PATH, or point %s at it.",
		GoToolBinary, GoToolModule, at, GoToolEnvVar,
	)
}

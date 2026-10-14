package build

import (
	"debug/elf"
	"fmt"
	"os"
	"path/filepath"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"go.uber.org/zap"
)

const (
	// MaintenanceRunnerBinaryEnvVar names a prebuilt maintenance runner binary
	// to package.
	//
	// Until releases publish the runner, this is how one is supplied. The
	// resolver is written to take a second source without anything downstream
	// changing, since the build manifest records an artefact either way.
	MaintenanceRunnerBinaryEnvVar = "CELERITY_MAINTENANCE_RUNNER_BINARY"

	// maintenanceRunnerEntryName is what the binary must be called inside the
	// archive. Lambda's provided.al2 runtimes execute a file named bootstrap,
	// and nothing else.
	maintenanceRunnerEntryName = "bootstrap"

	// sqlDatabaseResourceType is the abstract resource whose presence calls for
	// a runner.
	sqlDatabaseResourceType = "celerity/sqlDatabase"

	// maintenanceRunnerGoArch is the GOARCH the runner must be built for, and
	// must match the architecture the transformer declares on the function.
	maintenanceRunnerGoArch = "arm64"
)

// maintenanceRunnerGoArch as it appears in an ELF header.
const maintenanceRunnerELFMachine = elf.EM_AARCH64

// Returns nil when the application has no SQL database to maintain.
//
// The runner is a compiled Go binary versioned with the CLI rather than with
// the user's source, so it is neither the app bundle nor a dependency layer but
// its own artefact with its own content hash.
func (s *LambdaStrategy) buildMaintenanceRunner(params *BuildParams) (*Artifact, error) {
	if !blueprintHasSQLDatabase(params.Blueprint) {
		return nil, nil
	}

	binaryPath, err := resolveMaintenanceRunnerBinary()
	if err != nil {
		return nil, err
	}

	params.Logger.Info("packaging the database maintenance runner",
		zap.String("binary", binaryPath))

	stagingDir, err := os.MkdirTemp("", "celerity-maintenance-runner-*")
	if err != nil {
		return nil, fmt.Errorf("creating maintenance runner staging directory: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	staged := filepath.Join(stagingDir, maintenanceRunnerEntryName)
	if err := copyFile(binaryPath, staged); err != nil {
		return nil, fmt.Errorf("staging the maintenance runner binary: %w", err)
	}
	// The archive must carry the executable bit or the function fails at start
	// with a permission error that says nothing about the cause. copyFile does
	// not preserve modes, so it is set here rather than assumed.
	if err := os.Chmod(staged, 0755); err != nil {
		return nil, fmt.Errorf("making the maintenance runner binary executable: %w", err)
	}

	return writeArtifact(ArtifactTypeLambdaArchive, stagingDir, params.OutputDir, "maintenance-runner")
}

func resolveMaintenanceRunnerBinary() (string, error) {
	path := os.Getenv(MaintenanceRunnerBinaryEnvVar)
	if path == "" {
		return "", fmt.Errorf(
			"this application has a %s, which needs the database maintenance runner, "+
				"but no runner binary was found.\n"+
				"Build one and point %s at it:\n"+
				"  GOOS=linux GOARCH=arm64 go build -o bootstrap ./cmd/maintenance-runner",
			sqlDatabaseResourceType, MaintenanceRunnerBinaryEnvVar)
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s points at %s, which cannot be read: %w",
			MaintenanceRunnerBinaryEnvVar, path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s points at %s, which is a directory, not a binary",
			MaintenanceRunnerBinaryEnvVar, path)
	}
	if err := checkMaintenanceRunnerArchitecture(path); err != nil {
		return "", err
	}
	return path, nil
}

// Rejects a binary Lambda will not be able to execute.
//
// The transformer declares the runner function as arm64, and a binary built for
// anything else is deployed without complaint and then fails on first
// invocation with "Runtime exited with error: exit status 126". That message
// names neither the architecture nor the file, and reads the same as a binary
// missing its executable bit, so the mismatch is caught here, while the
// packaging step still has the file in hand.
func checkMaintenanceRunnerArchitecture(path string) error {
	file, err := elf.Open(path)
	if err != nil {
		return fmt.Errorf(
			"%s points at %s, which is not a Linux executable: %w.\n"+
				"Build the runner for Lambda with:\n"+
				"  GOOS=linux GOARCH=%s go build -o bootstrap ./cmd/maintenance-runner",
			MaintenanceRunnerBinaryEnvVar, path, err, maintenanceRunnerGoArch)
	}
	defer file.Close()

	if file.Machine != maintenanceRunnerELFMachine {
		return fmt.Errorf(
			"%s points at %s, which is built for %s, but the runner function is "+
				"deployed as %s.\n"+
				"Rebuild it with:\n"+
				"  GOOS=linux GOARCH=%s go build -o bootstrap ./cmd/maintenance-runner",
			MaintenanceRunnerBinaryEnvVar, path, file.Machine, maintenanceRunnerGoArch,
			maintenanceRunnerGoArch)
	}
	return nil
}

// An application with no SQL database gets no runner, and so no function to pay
// for, patch or reason about.
func blueprintHasSQLDatabase(bp *schema.Blueprint) bool {
	if bp == nil || bp.Resources == nil {
		return false
	}
	for _, resource := range bp.Resources.Values {
		if resource != nil && resource.Type != nil && resource.Type.Value == sqlDatabaseResourceType {
			return true
		}
	}
	return false
}

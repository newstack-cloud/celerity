package build

import (
	"archive/zip"
	"debug/elf"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func blueprintWith(types ...string) *schema.Blueprint {
	resources := map[string]*schema.Resource{}
	for i, typ := range types {
		resources[string(rune('a'+i))] = &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: typ},
		}
	}
	return &schema.Blueprint{Resources: &schema.ResourceMap{Values: resources}}
}

func runnerParams(t *testing.T, bp *schema.Blueprint) *BuildParams {
	t.Helper()
	return &BuildParams{
		Blueprint: bp,
		OutputDir: t.TempDir(),
		Logger:    zap.NewNop(),
	}
}

// A prebuilt binary, standing in for the cross-compiled runner.
//
// The header has to be a real one rather than a marker string, because
// packaging checks the architecture. Lambda cannot execute a binary built for
// the wrong one and reports that only as "exit status 126" at invocation.
func fakeRunnerBinary(t *testing.T) string {
	t.Helper()
	return runnerBinaryFor(t, elf.EM_AARCH64, "fake runner")
}

// Writes a file with a valid 64-bit little-endian ELF header for the given machine,
// followed by arbitrary content so that distinct binaries hash differently.
func runnerBinaryFor(t *testing.T, machine elf.Machine, content string) string {
	t.Helper()

	header := make([]byte, 64)
	copy(header, []byte{0x7f, 'E', 'L', 'F'})
	header[4] = byte(elf.ELFCLASS64)
	header[5] = byte(elf.ELFDATA2LSB)
	header[6] = byte(elf.EV_CURRENT)
	binary.LittleEndian.PutUint16(header[16:], uint16(elf.ET_EXEC))
	binary.LittleEndian.PutUint16(header[18:], uint16(machine))
	binary.LittleEndian.PutUint32(header[20:], uint32(elf.EV_CURRENT))
	// Section and program header tables are absent, which debug/elf accepts.
	binary.LittleEndian.PutUint16(header[52:], 64)

	path := filepath.Join(t.TempDir(), "bootstrap")
	require.NoError(t, os.WriteFile(path, append(header, []byte(content)...), 0755))
	return path
}

// An application with no database gets no runner, and so nothing to pay for,
// patch or reason about.
func TestNoRunnerWithoutASQLDatabase(t *testing.T) {
	params := runnerParams(t, blueprintWith("celerity/handler", "celerity/bucket"))

	artifact, err := (&LambdaStrategy{}).buildMaintenanceRunner(params)

	require.NoError(t, err)
	assert.Nil(t, artifact)
}

// And crucially, no error either. The missing-binary error must not fire for
// the applications that will never need one.
func TestNoDatabaseMeansNoMissingBinaryError(t *testing.T) {
	t.Setenv(MaintenanceRunnerBinaryEnvVar, "")
	params := runnerParams(t, blueprintWith("celerity/handler"))

	_, err := (&LambdaStrategy{}).buildMaintenanceRunner(params)

	require.NoError(t, err)
}

func TestPackagesTheRunnerForADatabaseBackedApp(t *testing.T) {
	t.Setenv(MaintenanceRunnerBinaryEnvVar, fakeRunnerBinary(t))
	params := runnerParams(t, blueprintWith("celerity/handler", "celerity/sqlDatabase"))

	artifact, err := (&LambdaStrategy{}).buildMaintenanceRunner(params)

	require.NoError(t, err)
	require.NotNil(t, artifact)
	assert.Equal(t, ArtifactTypeLambdaArchive, artifact.Type)
	assert.Contains(t, artifact.ContentHash, "sha256:")
	assert.FileExists(t, artifact.LocalPath)
}

// Lambda's provided.al2 runtimes execute a file named bootstrap and nothing
// else, and only if it is executable. Neither is visible until the function is
// invoked, where it surfaces as a start-up error that names neither cause.
func TestTheArchiveHoldsAnExecutableBootstrap(t *testing.T) {
	t.Setenv(MaintenanceRunnerBinaryEnvVar, fakeRunnerBinary(t))
	params := runnerParams(t, blueprintWith("celerity/sqlDatabase"))

	artifact, err := (&LambdaStrategy{}).buildMaintenanceRunner(params)
	require.NoError(t, err)

	reader, err := zip.OpenReader(artifact.LocalPath)
	require.NoError(t, err)
	defer reader.Close()

	require.Len(t, reader.File, 1, "the archive holds the binary and nothing else")
	entry := reader.File[0]
	assert.Equal(t, "bootstrap", entry.Name)
	assert.NotZero(t, entry.Mode()&0111, "bootstrap must be executable in the archive")
}

// A rebuilt binary must produce a different artefact, or a deploy silently
// keeps the old runner.
func TestTheArtefactIsContentAddressed(t *testing.T) {
	first := fakeRunnerBinary(t)
	t.Setenv(MaintenanceRunnerBinaryEnvVar, first)
	params := runnerParams(t, blueprintWith("celerity/sqlDatabase"))

	before, err := (&LambdaStrategy{}).buildMaintenanceRunner(params)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(first, mustReadFile(t, runnerBinaryFor(t, elf.EM_AARCH64, "a newer runner")), 0755))
	after, err := (&LambdaStrategy{}).buildMaintenanceRunner(params)
	require.NoError(t, err)

	assert.NotEqual(t, before.ContentHash, after.ContentHash)
}

// The error an operator is most likely to hit names the command to run.
func TestAMissingBinaryExplainsHowToBuildOne(t *testing.T) {
	t.Setenv(MaintenanceRunnerBinaryEnvVar, "")
	params := runnerParams(t, blueprintWith("celerity/sqlDatabase"))

	_, err := (&LambdaStrategy{}).buildMaintenanceRunner(params)

	require.Error(t, err)
	assert.Contains(t, err.Error(), MaintenanceRunnerBinaryEnvVar)
	assert.Contains(t, err.Error(), "go build -o bootstrap ./cmd/maintenance-runner")
}

func TestAnUnreadableBinaryIsReported(t *testing.T) {
	t.Setenv(MaintenanceRunnerBinaryEnvVar, filepath.Join(t.TempDir(), "absent"))
	params := runnerParams(t, blueprintWith("celerity/sqlDatabase"))

	_, err := (&LambdaStrategy{}).buildMaintenanceRunner(params)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be read")
}

func TestADirectoryIsNotABinary(t *testing.T) {
	t.Setenv(MaintenanceRunnerBinaryEnvVar, t.TempDir())
	params := runnerParams(t, blueprintWith("celerity/sqlDatabase"))

	_, err := (&LambdaStrategy{}).buildMaintenanceRunner(params)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "a directory, not a binary")
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

// A binary for the wrong architecture deploys without complaint and fails only when
// the function is first invoked, as "exit status 126", which names neither the
// architecture nor the file.
func TestPackagingRejectsARunnerBuiltForTheWrongArchitecture(t *testing.T) {
	t.Setenv(MaintenanceRunnerBinaryEnvVar, runnerBinaryFor(t, elf.EM_X86_64, "amd64 runner"))

	_, err := resolveMaintenanceRunnerBinary()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "built for")
	assert.Contains(t, err.Error(), "GOARCH=arm64")
}

package build

import (
	"archive/zip"
	"context"
	"debug/elf"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// compiledStubToolchain stands in for a toolchain that writes a binary.
type compiledStubToolchain struct {
	built []handlertoolchain.BinaryParams
	// machine, when set, makes the stub write an ELF header declaring it.
	machine elf.Machine
	// skipOutput makes the stub report success without writing anything.
	skipOutput bool
	buildErr   error
}

func (t *compiledStubToolchain) Name() string                        { return "stub" }
func (t *compiledStubToolchain) Validate(string) error               { return nil }
func (t *compiledStubToolchain) TestCommand(string) ([]string, bool) { return nil, false }

func (t *compiledStubToolchain) Prepare(context.Context, handlertoolchain.PrepareParams) error {
	return nil
}

func (t *compiledStubToolchain) Extract(
	context.Context, handlertoolchain.ExtractParams,
) (*preprocess.HandlerManifest, error) {
	return nil, nil
}

func (t *compiledStubToolchain) WatchPaths() handlertoolchain.WatchPaths {
	return handlertoolchain.WatchPaths{}
}

func (t *compiledStubToolchain) BuildBinary(
	_ context.Context, p handlertoolchain.BinaryParams,
) error {
	t.built = append(t.built, p)
	if t.buildErr != nil {
		return t.buildErr
	}
	if t.skipOutput {
		return nil
	}
	return os.WriteFile(p.OutputPath, elfHeader(t.machineOr(elf.EM_AARCH64)), 0o755)
}

func (t *compiledStubToolchain) machineOr(fallback elf.Machine) elf.Machine {
	if t.machine != 0 {
		return t.machine
	}
	return fallback
}

// elfHeader is a 64-byte ELF64 header declaring the given machine, which is
// all the architecture check reads.
func elfHeader(machine elf.Machine) []byte {
	header := make([]byte, 64)
	copy(header, []byte{0x7f, 'E', 'L', 'F'})
	header[4] = 2
	header[5] = 1
	header[6] = 1
	binary.LittleEndian.PutUint16(header[16:], uint16(elf.ET_EXEC))
	binary.LittleEndian.PutUint16(header[18:], uint16(machine))
	binary.LittleEndian.PutUint32(header[20:], 1)
	binary.LittleEndian.PutUint16(header[52:], 64)
	return header
}

type LambdaCompiledSuite struct {
	suite.Suite
	appDir    string
	outputDir string
	toolchain *compiledStubToolchain
}

func (s *LambdaCompiledSuite) SetupTest() {
	s.appDir = s.T().TempDir()
	s.outputDir = s.T().TempDir()
	s.toolchain = &compiledStubToolchain{}
}

func (s *LambdaCompiledSuite) build() (*Manifest, error) {
	strategy := &LambdaStrategy{Registry: NewRegistry()}
	return strategy.Build(context.Background(), &BuildParams{
		AppDir:       s.appDir,
		Runtime:      "os-only.debian13",
		DeployTarget: "aws-serverless",
		OutputDir:    s.outputDir,
		Logger:       zap.NewNop(),
		Toolchain:    s.toolchain,
	})
}

// zipEntries reports the archive's entries and whether each is executable.
func (s *LambdaCompiledSuite) zipEntries(path string) map[string]bool {
	r, err := zip.OpenReader(path)
	s.Require().NoError(err)
	defer r.Close()

	entries := map[string]bool{}
	for _, f := range r.File {
		entries[f.Name] = f.Mode()&0o111 != 0
	}
	return entries
}

// Nothing is registered for an OS-only runtime, and the interpreted path errors
// when its dependency installer lookup finds none, so the compiled branch has
// to be taken before that lookup.
func (s *LambdaCompiledSuite) Test_Build_does_not_need_a_dependency_installer() {
	manifest, err := s.build()
	s.Require().NoError(err)
	s.Require().NotNil(manifest.Lambda)
	s.Assert().Nil(manifest.Lambda.SharedLayer, "a compiled language has no dependency layer")
}

func (s *LambdaCompiledSuite) Test_Build_archives_only_the_bootstrap_and_the_sdk_files() {
	manifest, err := s.build()
	s.Require().NoError(err)

	entries := s.zipEntries(manifest.Lambda.AppCode.LocalPath)
	s.Assert().Len(entries, 3)
	s.Assert().Contains(entries, "bootstrap")
	s.Assert().Contains(entries, ResourceLinksFilename)
	s.Assert().Contains(entries, HandlerScopeFilename)
}

// The platform executes the file, so the bit has to survive the archive.
func (s *LambdaCompiledSuite) Test_Build_keeps_the_bootstrap_executable_in_the_archive() {
	manifest, err := s.build()
	s.Require().NoError(err)

	entries := s.zipEntries(manifest.Lambda.AppCode.LocalPath)
	s.Assert().True(entries["bootstrap"], "expected bootstrap to be executable inside the archive")
}

// Source is not copied, because the archive carries a binary rather than the
// code it was built from.
func (s *LambdaCompiledSuite) Test_Build_does_not_copy_application_source() {
	s.Require().NoError(os.WriteFile(filepath.Join(s.appDir, "main.go"), []byte("package main"), 0o644))

	manifest, err := s.build()
	s.Require().NoError(err)

	entries := s.zipEntries(manifest.Lambda.AppCode.LocalPath)
	s.Assert().NotContains(entries, "main.go")
}

// A custom runtime executes bootstrap and dispatches on an environment
// variable, so the transformer needs the runtime and architecture rather than
// a function name.
func (s *LambdaCompiledSuite) Test_Build_records_what_the_transformer_declares_the_function_from() {
	manifest, err := s.build()
	s.Require().NoError(err)

	s.Assert().Equal("bootstrap", manifest.Lambda.EntryPoint)
	s.Assert().Equal("arm64", manifest.Lambda.Architecture)
	s.Assert().Equal("provided.al2023", manifest.Lambda.LambdaRuntime)
	s.Assert().Equal("os-only.debian13", manifest.Runtime)
	s.Assert().Equal("aws-serverless", manifest.Target)
}

func (s *LambdaCompiledSuite) Test_Build_asks_for_a_linux_binary_of_the_lambda_architecture() {
	_, err := s.build()
	s.Require().NoError(err)

	s.Require().Len(s.toolchain.built, 1)
	built := s.toolchain.built[0]
	s.Assert().Equal("linux", built.OS)
	s.Assert().Equal("arm64", built.Arch)
	s.Assert().Equal("bootstrap", filepath.Base(built.OutputPath))
	s.Assert().Equal(handlertoolchain.ModeDeploy, built.Mode)
}

// A custom build command is free to produce whatever it likes, and this is the
// last point before the archive is sealed.
func (s *LambdaCompiledSuite) Test_Build_refuses_a_binary_of_the_wrong_architecture() {
	s.toolchain.machine = elf.EM_X86_64

	_, err := s.build()
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "arm64")
}

func (s *LambdaCompiledSuite) Test_Build_reports_a_toolchain_that_produced_no_executable() {
	s.toolchain.skipOutput = true

	_, err := s.build()
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "left no executable")
}

func (s *LambdaCompiledSuite) Test_Build_propagates_a_build_failure() {
	s.toolchain.buildErr = errors.New("go build failed")

	_, err := s.build()
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "go build failed")
}

func (s *LambdaCompiledSuite) Test_Build_artifact_is_content_hashed_and_sized() {
	manifest, err := s.build()
	s.Require().NoError(err)

	art := manifest.Lambda.AppCode
	s.Assert().Equal(ArtifactTypeLambdaArchive, art.Type)
	s.Assert().Regexp(`^sha256:[0-9a-f]{8,}$`, art.ContentHash)
	s.Assert().Positive(art.Size)
	s.Assert().Contains(filepath.Base(art.LocalPath), "handlers-")
}

func TestLambdaCompiledSuite(t *testing.T) { suite.Run(t, new(LambdaCompiledSuite)) }

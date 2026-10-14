package handlertoolchain

import (
	"debug/elf"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ArchitectureSuite struct {
	suite.Suite
	dir string
}

func (s *ArchitectureSuite) SetupTest() { s.dir = s.T().TempDir() }

// writeELF writes a 64-byte ELF64 header declaring the given machine. Enough
// for debug/elf to read the architecture, which is all the check looks at.
func (s *ArchitectureSuite) writeELF(name string, machine elf.Machine) string {
	header := make([]byte, 64)
	copy(header, []byte{0x7f, 'E', 'L', 'F'})
	header[4] = 2 // ELFCLASS64
	header[5] = 1 // little endian
	header[6] = 1 // version
	binary.LittleEndian.PutUint16(header[16:], uint16(elf.ET_EXEC))
	binary.LittleEndian.PutUint16(header[18:], uint16(machine))
	binary.LittleEndian.PutUint32(header[20:], 1)
	binary.LittleEndian.PutUint16(header[52:], 64) // e_ehsize

	path := filepath.Join(s.dir, name)
	s.Require().NoError(os.WriteFile(path, header, executableMode))
	return path
}

func (s *ArchitectureSuite) Test_matching_architecture_passes() {
	s.Assert().NoError(CheckArchitecture(s.writeELF("arm", elf.EM_AARCH64), "arm64"))
	s.Assert().NoError(CheckArchitecture(s.writeELF("x86", elf.EM_X86_64), "amd64"))
}

// The kernel's refusal to exec a foreign binary surfaces as exit status 126
// with nothing to say why, so the error has to supply the why itself.
func (s *ArchitectureSuite) Test_mismatched_architecture_names_both_sides() {
	err := CheckArchitecture(s.writeELF("arm", elf.EM_AARCH64), "amd64")
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "arm64")
	s.Assert().Contains(err.Error(), "amd64")
	s.Assert().Contains(err.Error(), "126")
}

// A non-ELF file means a build for somewhere the check does not apply, not a
// mistake.
func (s *ArchitectureSuite) Test_non_elf_file_is_not_an_error() {
	path := filepath.Join(s.dir, "mach-o")
	s.Require().NoError(os.WriteFile(path, []byte("not an elf file"), executableMode))
	s.Assert().NoError(CheckArchitecture(path, "arm64"))
}

func (s *ArchitectureSuite) Test_unknown_goarch_is_not_checked() {
	s.Assert().NoError(CheckArchitecture(s.writeELF("arm", elf.EM_AARCH64), "riscv64"))
}

func (s *ArchitectureSuite) Test_copyExecutable_sets_the_execute_bit_regardless_of_the_source() {
	src := filepath.Join(s.dir, "src")
	s.Require().NoError(os.WriteFile(src, []byte("binary"), 0o600))

	dst := filepath.Join(s.dir, "nested", "dst")
	s.Require().NoError(copyExecutable(src, dst))

	info, err := os.Stat(dst)
	s.Require().NoError(err)
	s.Assert().Equal(executableMode, info.Mode().Perm())
}

func TestArchitectureSuite(t *testing.T) { suite.Run(t, new(ArchitectureSuite)) }

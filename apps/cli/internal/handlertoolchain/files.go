package handlertoolchain

import (
	"debug/elf"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// The core runtime starts the handlers executable as a child process, so it
// has to carry the execute bit through every copy and archive on the way.
const executableMode os.FileMode = 0o755

func sameFile(a, b string) bool {
	infoA, err := os.Stat(a)
	if err != nil {
		return false
	}
	infoB, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(infoA, infoB)
}

func ensureExecutable(path string) error {
	if err := os.Chmod(path, executableMode); err != nil {
		return fmt.Errorf("making %s executable: %w", filepath.Base(path), err)
	}
	return nil
}

// The execute bit is set explicitly rather than carried over, because the mode
// a copy inherits depends on the umask and an executable that arrives without
// it fails as a bare "permission denied" from the supervisor.
func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, executableMode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return ensureExecutable(dst)
}

// CheckArchitecture reports whether the executable at path is built for the
// given GOARCH.
//
// Worth a check of its own because the failure it catches is otherwise
// invisible. An executable of the wrong architecture is a valid file the
// kernel refuses to exec, which surfaces as "exit status 126" with no
// indication of why, long after the build that caused it.
//
// A non-ELF file is not an error. A Lambda or container target is always
// Linux, so anything else is a build for somewhere the check does not apply
// rather than a mistake.
func CheckArchitecture(path, goarch string) error {
	file, err := elf.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	want, ok := elfMachines[goarch]
	if !ok {
		return nil
	}
	if file.Machine != want {
		return fmt.Errorf(
			"the handlers executable at %s is built for %s, and this target needs %s.\n"+
				"The kernel refuses to run it, which shows up as exit status 126 with nothing else to go on",
			path, machineGOARCH(file.Machine), goarch,
		)
	}
	return nil
}

var elfMachines = map[string]elf.Machine{
	"arm64": elf.EM_AARCH64,
	"amd64": elf.EM_X86_64,
}

func machineGOARCH(machine elf.Machine) string {
	for goarch, m := range elfMachines {
		if m == machine {
			return goarch
		}
	}
	return machine.String()
}

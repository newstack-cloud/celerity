package docker

import (
	"context"
	"fmt"
)

// DaemonArchitecture reports the architecture the Docker daemon runs
// containers at, as a GOARCH name.
//
// Read from the daemon rather than taken from the CLI's own GOARCH, because the
// two are often different. A remote context, or a Linux VM on an arm64 Mac, runs
// containers at an architecture the client is not built for, and a handlers
// executable cross-compiled for the wrong one is a valid file the kernel refuses
// to exec, which the supervisor reports as a bare ENOEXEC.
func (r *RuntimeContainer) DaemonArchitecture(ctx context.Context) (string, error) {
	info, err := r.client.Info(ctx)
	if err != nil {
		return "", fmt.Errorf("asking the Docker daemon for its architecture: %w", err)
	}
	return NormaliseArchitecture(info.Architecture), nil
}

// NormaliseArchitecture maps the architecture names Docker reports onto GOARCH
// names.
//
// Docker reports the kernel's own spelling (uname -m), which is not what Go
// calls the same architectures.
func NormaliseArchitecture(arch string) string {
	switch arch {
	case "aarch64", "arm64", "arm64/v8":
		return "arm64"
	case "x86_64", "amd64":
		return "amd64"
	default:
		// Passed through rather than rejected, because an architecture neither
		// side has a translation for is still the right answer to hand a
		// compiler, and the build fails plainly enough if it is not.
		return arch
	}
}

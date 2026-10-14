package handlertoolchain

import (
	"fmt"
	"strings"

	"github.com/newstack-cloud/celerity/apps/cli/internal/consts"
)

// GoSDKModule is the Celerity Go SDK's module path. A go.mod requiring it is
// what identifies a Go Celerity application.
const GoSDKModule = "github.com/newstack-cloud/celerity-go-sdk"

// Options configures toolchain resolution.
type Options struct {
	// Runtime is the blueprint's runtime identifier.
	Runtime string
	// AppDir is the application directory.
	AppDir string
	// Config is the handlerBuild block, or nil when the app declares none.
	Config *Config
	// Runner runs external commands. Defaults to ExecRunner.
	Runner Runner
}

// Resolve returns the toolchain for an application, or (nil, nil) where the
// runtime is not an OS-only one.
//
// Returning nil rather than an error for Node.js and Python keeps every call
// site uniform. A caller threads whatever comes back, and the nil case means
// the runtime hosts the language itself and there is nothing to build.
//
// An explicit handlerBuild block wins over an inferred Go project, because a
// Go application that declares one has asked to drive its own build.
func Resolve(opts Options) (Toolchain, error) {
	if !IsOSOnly(opts.Runtime) {
		return nil, nil
	}

	runner := opts.Runner
	if runner == nil {
		runner = ExecRunner{}
	}

	if opts.Config != nil {
		if err := opts.Config.Validate(); err != nil {
			return nil, err
		}
		return newCustomToolchain(opts.Config, runner), nil
	}

	isGo, err := declaresGoSDK(opts.AppDir)
	if err != nil {
		return nil, err
	}
	if isGo {
		return newGoToolchain(runner), nil
	}

	return nil, fmt.Errorf(
		"runtime %q needs a handlers executable, and this project does not say how to build one.\n"+
			"For Go, require %s in go.mod. For another compiled language, add a handlerBuild "+
			"block to your deploy config naming the command that builds the executable",
		opts.Runtime, GoSDKModule,
	)
}

// IsOSOnly reports whether a runtime identifier names an OS-only runtime.
func IsOSOnly(runtime string) bool {
	return strings.HasPrefix(runtime, consts.RuntimeOSOnly)
}

func declaresGoSDK(appDir string) (bool, error) {
	goMod, err := readGoMod(appDir)
	if err != nil {
		return false, err
	}
	_, found := requires(goMod, GoSDKModule)
	return found, nil
}

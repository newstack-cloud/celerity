package devtest

import (
	"fmt"
	"strings"

	"github.com/newstack-cloud/celerity/apps/cli/internal/consts"
	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
	"go.uber.org/zap"
)

// RunnerFactory creates a TestRunner for a given runtime.
//
// The toolchain is nil for a runtime that hosts the language itself, where the
// test command is a convention of that language rather than something the
// project declares.
type RunnerFactory func(toolchain handlertoolchain.Toolchain, logger *zap.Logger) TestRunner

var runners = map[string]RunnerFactory{
	consts.LanguageNodeJS: func(_ handlertoolchain.Toolchain, l *zap.Logger) TestRunner {
		return NewNodeRunner(l)
	},
	consts.LanguagePython: func(_ handlertoolchain.Toolchain, l *zap.Logger) TestRunner {
		return NewPythonRunner(l)
	},
	consts.RuntimeOSOnly: func(t handlertoolchain.Toolchain, l *zap.Logger) TestRunner {
		return NewOSHandlerRunner(t, l)
	},
}

// RunnerForRuntime returns a TestRunner for the given runtime string,
// matching by longest prefix (e.g. "nodejs24.x" matches "nodejs").
func RunnerForRuntime(
	runtime string,
	toolchain handlertoolchain.Toolchain,
	logger *zap.Logger,
) (TestRunner, error) {
	var (
		best    RunnerFactory
		longest int
		found   bool
	)
	for prefix, factory := range runners {
		if !strings.HasPrefix(runtime, prefix) {
			continue
		}
		if !found || len(prefix) > longest {
			best, longest, found = factory, len(prefix), true
		}
	}
	if !found {
		return nil, fmt.Errorf(
			"no test runner for runtime %q; supported runtimes: nodejs, python, os-only",
			runtime,
		)
	}
	return best(toolchain, logger), nil
}

package devtest

import (
	"fmt"
	"strings"

	"github.com/newstack-cloud/celerity/apps/cli/internal/consts"
	"go.uber.org/zap"
)

// RunnerFactory creates a TestRunner for a given runtime.
type RunnerFactory func(logger *zap.Logger) TestRunner

var runners = map[string]RunnerFactory{
	consts.LanguageNodeJS: func(l *zap.Logger) TestRunner { return NewNodeRunner(l) },
	consts.LanguagePython: func(l *zap.Logger) TestRunner { return NewPythonRunner(l) },
}

// RunnerForRuntime returns a TestRunner for the given runtime string,
// matching by prefix (e.g. "nodejs24.x" matches "nodejs").
func RunnerForRuntime(runtime string, logger *zap.Logger) (TestRunner, error) {
	for prefix, factory := range runners {
		if strings.HasPrefix(runtime, prefix) {
			return factory(logger), nil
		}
	}
	return nil, fmt.Errorf(
		"no test runner for runtime %q; supported runtimes: nodejs, python",
		runtime,
	)
}

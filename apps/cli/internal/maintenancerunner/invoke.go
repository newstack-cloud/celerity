package maintenancerunner

import (
	"context"
	"fmt"

	"github.com/newstack-cloud/bluelink/libs/blueprint/state"
)

// Invoker calls a deployed maintenance runner and returns its response.
//
// One implementation per deploy target, selected by InvokerFor, for the same
// reason deploydb has one resolver per target: how a runner is addressed and
// invoked is entirely the provider's business, while what is asked of it is
// not.
type Invoker interface {
	// Invoke runs one operation against a deployed runner. It returns an error
	// only when the runner could not be reached; an operation that ran and
	// failed comes back as a Response with OK false.
	Invoke(ctx context.Context, instance *state.InstanceState, req Request) (*Response, error)

	// Deployed reports whether the instance has a runner at all. An instance
	// deployed before runners existed, or one whose database is publicly
	// reachable, does not.
	Deployed(instance *state.InstanceState) bool
}

// newInvoker is the per-target registry. Providers register into it from their
// own package so this file has no dependency on any of them.
var newInvoker = map[string]func() Invoker{}

// RegisterInvoker adds a deploy target's invoker. Called from a provider
// package's init.
func RegisterInvoker(deployTarget string, build func() Invoker) {
	newInvoker[deployTarget] = build
}

// InvokerFor returns the maintenance runner invoker for a deploy target.
func InvokerFor(deployTarget string) (Invoker, error) {
	if build, ok := newInvoker[deployTarget]; ok {
		return build(), nil
	}
	return nil, fmt.Errorf(
		"running database maintenance operations is not yet supported for deploy target %q",
		deployTarget)
}

// Annotations the transformer stamps on a deployed runner so the CLI can find
// it in instance state without knowing what the provider named it.
const (
	// RunnerAnnotation marks the concrete resource that is the runner.
	RunnerAnnotation = "celerity.maintenance.runner"

	// RunnerAnnotationValue is the value RunnerAnnotation carries.
	RunnerAnnotationValue = "true"
)

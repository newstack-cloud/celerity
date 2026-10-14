package maintenancerunner

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/newstack-cloud/celerity/apps/cli/internal/dbconn"
	"go.uber.org/zap"
)

// Handler receives one maintenance request and always returns a response.
type Handler func(ctx context.Context, req Request) *Response

// Host is the platform the runner runs on. It covers what delivers invocations
// to the runner and where credentials come from there.
//
// The runner binary itself names no provider. It registers whichever hosts were
// compiled into it, picks the one it is running on, and serves.
type Host interface {
	// Name identifies the host platform ("aws").
	Name() string

	// Detect reports whether this process is running on it. Implementations
	// should key on something the platform itself sets, not on configuration a
	// user could get wrong.
	Detect() bool

	// Credentials resolves database credentials from the platform's secret
	// store.
	Credentials() dbconn.CredentialSource

	// Serve delivers invocations to handle. It does not return in normal
	// operation.
	Serve(handle Handler, logger *zap.Logger) error
}

// HostEnvVar overrides host detection. It exists for the case detection cannot
// cover, running the runner somewhere that looks like nothing in particular,
// such as a container during a test.
const HostEnvVar = "CELERITY_MAINTENANCE_HOST"

var hosts = map[string]Host{}

// RegisterHost adds a host platform. Called from a provider package's init.
func RegisterHost(host Host) {
	hosts[host.Name()] = host
}

// The host to run on is the one named by HostEnvVar, or failing that the first
// one that detects itself.
func resolveHost() (Host, error) {
	if name := os.Getenv(HostEnvVar); name != "" {
		host, ok := hosts[name]
		if !ok {
			return nil, fmt.Errorf("%s is set to %q, which this runner was not built with (it has: %v)",
				HostEnvVar, name, registeredHostNames())
		}
		return host, nil
	}

	for _, host := range hosts {
		if host.Detect() {
			return host, nil
		}
	}
	return nil, fmt.Errorf(
		"could not tell which platform this runner is on (it was built with: %v); set %s",
		registeredHostNames(), HostEnvVar)
}

// HostNames returns the host platforms this binary was built with, sorted.
func HostNames() []string { return registeredHostNames() }

func registeredHostNames() []string {
	names := make([]string, 0, len(hosts))
	for name := range hosts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Run starts the maintenance runner. It does not return in normal operation.
//
// Targets are read once at startup rather than per invocation. They are
// deploy-time configuration, and a bad one should fail here rather than on
// whichever invocation happens to land on a fresh execution environment.
func Run(logger *zap.Logger) error {
	host, err := resolveHost()
	if err != nil {
		return err
	}

	targets, err := LoadTargets()
	if err != nil {
		return fmt.Errorf("the maintenance runner is misconfigured: %w", err)
	}

	logger.Info("maintenance runner starting",
		zap.String("host", host.Name()),
		zap.Strings("databases", targets.Names()),
	)

	executor := NewExecutor(targets, host.Credentials(), logger)
	return host.Serve(executor.Handle, logger)
}

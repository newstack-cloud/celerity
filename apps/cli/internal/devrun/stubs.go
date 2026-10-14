package devrun

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/newstack-cloud/celerity/apps/cli/internal/compose"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devstubs"
	"github.com/newstack-cloud/celerity/apps/cli/internal/seed"
	"go.uber.org/zap"
)

// Runs after startDependencies, so mountebank is already healthy, and before
// seedData.
//
// With hostMode set, for integration tests, stub URLs are seeded as localhost
// addresses because the test process runs on the host. Without it, for full and
// API mode, they use Docker network addresses reachable from the app container
// on the compose network.
func (o *Orchestrator) loadStubs(ctx context.Context, hostMode bool) error {
	if o.config.ComposeCfg == nil {
		return nil
	}

	stubServices := o.config.ComposeCfg.StubServices
	if len(stubServices) == 0 {
		return nil
	}

	hostEndpoints := o.hostEndpoints()
	mbAPIURL, ok := hostEndpoints[compose.EnvStubsAPIURL]
	if !ok {
		return nil
	}

	if err := waitForMountebank(ctx, mbAPIURL); err != nil {
		o.output.PrintWarning("Mountebank not ready", err)
		return nil
	}

	loaded, err := devstubs.LoadStubs(o.config.AppDir)
	if err != nil {
		o.output.PrintWarning("Failed to load stubs for imposters", err)
		return nil
	}

	imposters := devstubs.ToImposters(loaded)
	if err := devstubs.LoadImposters(ctx, mbAPIURL, imposters); err != nil {
		o.output.PrintWarning("Failed to load mountebank imposters", err)
		return nil
	}

	o.output.PrintStep(fmt.Sprintf("Loaded %d stub imposter(s) into mountebank", len(imposters)))

	// Seed stub service URLs into the config namespace so the application
	// reads them via config.get().
	if err := o.seedStubConfigValues(ctx, stubServices, hostMode); err != nil {
		o.output.PrintWarning("Failed to seed stub config values", err)
	}

	return nil
}

func (o *Orchestrator) seedStubConfigValues(ctx context.Context, stubServices []compose.StubServiceInfo, hostMode bool) error {
	hostEndpoints := o.hostEndpoints()
	configEndpoint, ok := hostEndpoints[compose.EnvConfigEndpoint]
	if !ok {
		return nil
	}

	seeder, err := seed.NewValkeyConfigSeeder(configEndpoint, o.logger)
	if err != nil {
		return fmt.Errorf("config seeder init: %w", err)
	}
	defer seeder.Close()

	configs := seed.CollectConfigResources(o.config.Blueprint)

	for _, svc := range stubServices {
		namespace, err := resolveStubConfigNamespace(svc, configs)
		if err != nil {
			return err
		}
		if namespace == "" {
			o.logger.Warn("no config namespace for stub service, skipping config seed",
				zap.String("service", svc.Name),
			)
			continue
		}

		var url string
		if hostMode {
			// Integration tests run on the host.
			url = fmt.Sprintf("http://localhost:%d", svc.HostPort)
		} else {
			// The app container resolves "stubs" through compose DNS.
			url = fmt.Sprintf("http://%s:%d", compose.ServiceNameStubs, svc.Port)
		}
		values := map[string]string{svc.ConfigKey: url}

		if err := seeder.SeedConfig(ctx, namespace, values); err != nil {
			o.logger.Warn("failed to seed stub config",
				zap.String("service", svc.Name),
				zap.String("configKey", svc.ConfigKey),
				zap.Error(err),
			)
			continue
		}

		o.logger.Debug("seeded stub config",
			zap.String("namespace", namespace),
			zap.String("key", svc.ConfigKey),
			zap.String("url", url),
		)
	}

	return nil
}

func waitForMountebank(ctx context.Context, baseURL string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(15 * time.Second)

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("mountebank not ready at %s after 15s", baseURL)
}

// A declared namespace wins, matched against the application's config resources
// by either the blueprint resource name or the store name, so an author can name
// whichever they think of it by.
//
// With nothing declared and exactly one config resource there is no choice to
// make, so that one is used. With several this is an error rather than a guess,
// because picking one seeds the URL into a store the application never reads and
// the only symptom is requests going to the real endpoint instead of the stub, a
// long way from the blueprint that caused it.
func resolveStubConfigNamespace(
	svc compose.StubServiceInfo,
	configs []seed.ConfigResourceInfo,
) (string, error) {
	if declared := svc.ConfigNamespace; declared != "" {
		for _, cfg := range configs {
			if cfg.ResourceName == declared || cfg.StoreName == declared {
				return cfg.StoreName, nil
			}
		}
		// Named a store this application does not declare. Passed through
		// rather than refused, since a store can exist without the blueprint
		// describing it.
		return declared, nil
	}

	switch len(configs) {
	case 0:
		// Nothing to seed into. The caller skips the service.
		return "", nil
	case 1:
		return configs[0].StoreName, nil
	default:
		return "", fmt.Errorf(
			"stub service %q does not say which config store its URL belongs in, and this "+
				"application has %d: %s.\n"+
				"Set configNamespace on the stub service to one of them",
			svc.Name, len(configs), strings.Join(configStoreNames(configs), ", "),
		)
	}
}

func configStoreNames(configs []seed.ConfigResourceInfo) []string {
	names := make([]string, 0, len(configs))
	for _, cfg := range configs {
		names = append(names, cfg.StoreName)
	}
	return names
}

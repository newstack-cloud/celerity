package devrun

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/newstack-cloud/celerity/apps/cli/internal/compose"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devstubs"
	"github.com/newstack-cloud/celerity/apps/cli/internal/seed"
	"go.uber.org/zap"
)

// loadStubs loads mountebank imposters and seeds stub service URLs into
// the config namespace. This runs after startDependencies (mountebank is
// healthy) and before seedData.
//
// When hostMode is true (integration tests), stub URLs are seeded as
// localhost addresses since the test process runs on the host.
// When false (full/API mode), URLs use Docker network addresses since
// the app container runs on the compose network.
func (o *Orchestrator) loadStubs(ctx context.Context, hostMode bool) error {
	stubServices := o.config.ComposeCfg.StubServices
	if len(stubServices) == 0 {
		return nil
	}

	hostEndpoints := o.hostEndpoints()
	mbAPIURL, ok := hostEndpoints[compose.EnvStubsAPIURL]
	if !ok {
		return nil
	}

	// Wait for mountebank to be ready.
	if err := waitForMountebank(ctx, mbAPIURL); err != nil {
		o.output.PrintWarning("Mountebank not ready", err)
		return nil
	}

	// Load imposters from stub files.
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

	for _, svc := range stubServices {
		namespace := svc.ConfigNamespace
		if namespace == "" {
			// Default to the first config resource's store name.
			configs := seed.CollectConfigResources(o.config.Blueprint)
			if len(configs) > 0 {
				namespace = configs[0].StoreName
			}
		}
		if namespace == "" {
			o.logger.Warn("no config namespace for stub service, skipping config seed",
				zap.String("service", svc.Name),
			)
			continue
		}

		var url string
		if hostMode {
			// Host URL — integration tests run on the host.
			url = fmt.Sprintf("http://localhost:%d", svc.HostPort)
		} else {
			// Docker network URL — the app container resolves "stubs" via compose DNS.
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

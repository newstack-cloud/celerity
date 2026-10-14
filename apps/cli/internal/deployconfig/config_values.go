package deployconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/celerity/apps/cli/internal/seed"
)

// Fills each celerity/config resource's spec.values from the target
// environment's plaintext config directory, so a first deploy stands the store
// up with values already in it.
//
// Compute reads its configuration as it starts. Creating the store empty and
// importing afterwards leaves a window where it is missing, which a Lambda
// survives and a container does not, since a task that cannot read required
// config crashes and its service never stabilises. Seeding closes that window.
//
// This is a bootstrap, not a source of truth. The store's values are create-only
// on the provider side, so `celerity config set` remains the way configuration
// changes after the first deploy and a later deploy will not revert it.
//
// Secrets are seeded on the same terms, because a workload missing its database
// password is down rather than degraded. The values pass through the merged
// blueprint, which is written 0600 and gitignored, and are held in the deployed
// store from then on.
//
// A value written in the blueprint wins, being the more specific statement.
//
// Directories are per environment with no fallback, since falling back to
// config/local would push a developer's machine values into a deployed
// environment.
func seedConfigValues(bp *schema.Blueprint, appDir string, appEnv string) (int, error) {
	if bp == nil || appEnv == "" {
		return 0, nil
	}

	configDir := existingDir(filepath.Join(appDir, "config", appEnv))
	secretsDir := existingDir(filepath.Join(appDir, "secrets", appEnv))
	if configDir == "" && secretsDir == "" {
		return 0, nil
	}

	seeded := 0
	for _, info := range seed.CollectConfigResources(bp) {
		values, err := seed.LoadAndMergeConfig(configDir, secretsDir, info.StoreName)
		if err != nil {
			return 0, fmt.Errorf("loading config values for %q: %w", info.StoreName, err)
		}
		if len(values) == 0 {
			continue
		}

		resource := bp.Resources.Values[info.ResourceName]
		if resource == nil {
			continue
		}
		seeded += applyConfigValues(resource, values)
	}

	return seeded, nil
}

// Writes the loaded values into the resource's spec.values, leaving any key the
// blueprint already states alone.
func applyConfigValues(resource *schema.Resource, values map[string]string) int {
	if resource.Spec == nil {
		resource.Spec = &core.MappingNode{Fields: map[string]*core.MappingNode{}}
	}
	if resource.Spec.Fields == nil {
		resource.Spec.Fields = map[string]*core.MappingNode{}
	}

	existing, ok := resource.Spec.Fields["values"]
	if !ok || existing == nil || existing.Fields == nil {
		existing = &core.MappingNode{Fields: map[string]*core.MappingNode{}}
		resource.Spec.Fields["values"] = existing
	}

	applied := 0
	for key, value := range values {
		if _, declared := existing.Fields[key]; declared {
			continue
		}
		existing.Fields[key] = core.MappingNodeFromString(value)
		applied++
	}

	return applied
}

func existingDir(path string) string {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return ""
	}
	return path
}

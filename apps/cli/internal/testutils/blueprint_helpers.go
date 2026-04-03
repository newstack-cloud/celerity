package testutils

import (
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/require"
)

// LoadTestBlueprint loads a blueprint from a YAML string, failing the test on error.
func LoadTestBlueprint(t *testing.T, yamlContent string) *schema.Blueprint {
	t.Helper()
	bp, err := schema.LoadString(yamlContent, schema.YAMLSpecFormat)
	require.NoError(t, err, "failed to load test blueprint")
	return bp
}

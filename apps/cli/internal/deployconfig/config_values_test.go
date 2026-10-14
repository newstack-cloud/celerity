package deployconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/suite"
)

type ConfigValuesSuite struct {
	suite.Suite
}

func (s *ConfigValuesSuite) appDir(files map[string]string) string {
	dir := s.T().TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		s.Require().NoError(os.MkdirAll(filepath.Dir(path), 0o755))
		s.Require().NoError(os.WriteFile(path, []byte(content), 0o600))
	}
	return dir
}

func configBlueprint(storeName string, values map[string]string) *schema.Blueprint {
	spec := &core.MappingNode{
		Fields: map[string]*core.MappingNode{
			"name": core.MappingNodeFromString(storeName),
		},
	}
	if values != nil {
		declared := &core.MappingNode{Fields: map[string]*core.MappingNode{}}
		for key, value := range values {
			declared.Fields[key] = core.MappingNodeFromString(value)
		}
		spec.Fields["values"] = declared
	}

	return &schema.Blueprint{
		Resources: &schema.ResourceMap{
			Values: map[string]*schema.Resource{
				"appConfig": {
					Type: &schema.ResourceTypeWrapper{Value: "celerity/config"},
					Spec: spec,
				},
			},
		},
	}
}

func specValue(bp *schema.Blueprint, key string) string {
	values := bp.Resources.Values["appConfig"].Spec.Fields["values"]
	if values == nil {
		return ""
	}
	return core.StringValue(values.Fields[key])
}

// A store is created holding its values so that compute starting up finds its
// configuration already there.
func (s *ConfigValuesSuite) Test_values_come_from_the_environment_config_directory() {
	appDir := s.appDir(map[string]string{
		"config/production/appConfig.yaml": "APP_NAME: \"shop\"\nLOG_LEVEL: \"warn\"\n",
	})
	bp := configBlueprint("appConfig", nil)

	seeded, err := seedConfigValues(bp, appDir, "production")

	s.NoError(err)
	s.Equal(2, seeded)
	s.Equal("shop", specValue(bp, "APP_NAME"))
	s.Equal("warn", specValue(bp, "LOG_LEVEL"))
}

// A workload missing its database password is down, not degraded, so secrets
// are bootstrapped on the same terms as plaintext values.
func (s *ConfigValuesSuite) Test_secrets_are_seeded_alongside_plaintext_values() {
	appDir := s.appDir(map[string]string{
		"config/production/appConfig.yaml":  "LOG_LEVEL: \"warn\"\n",
		"secrets/production/appConfig.yaml": "API_TOKEN: \"from-secrets\"\n",
	})
	bp := configBlueprint("appConfig", nil)

	seeded, err := seedConfigValues(bp, appDir, "production")

	s.NoError(err)
	s.Equal(2, seeded)
	s.Equal("warn", specValue(bp, "LOG_LEVEL"))
	s.Equal("from-secrets", specValue(bp, "API_TOKEN"))
}

// The blueprint is the more specific statement, so it is not overwritten.
func (s *ConfigValuesSuite) Test_a_value_written_in_the_blueprint_wins() {
	appDir := s.appDir(map[string]string{
		"config/production/appConfig.yaml": "LOG_LEVEL: \"warn\"\n",
	})
	bp := configBlueprint("appConfig", map[string]string{"LOG_LEVEL": "debug"})

	seeded, err := seedConfigValues(bp, appDir, "production")

	s.NoError(err)
	s.Equal(0, seeded)
	s.Equal("debug", specValue(bp, "LOG_LEVEL"))
}

// Falling back to config/local would push a developer's machine values into a
// deployed environment.
func (s *ConfigValuesSuite) Test_another_environments_values_are_not_used() {
	appDir := s.appDir(map[string]string{
		"config/local/appConfig.yaml": "LOG_LEVEL: \"debug\"\n",
	})
	bp := configBlueprint("appConfig", nil)

	seeded, err := seedConfigValues(bp, appDir, "production")

	s.NoError(err)
	s.Equal(0, seeded)
	s.Equal("", specValue(bp, "LOG_LEVEL"))
}

func TestConfigValuesSuite(t *testing.T) {
	suite.Run(t, new(ConfigValuesSuite))
}

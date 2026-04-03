package seed

import (
	"os"
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/suite"
)

type ConfigSeederTestSuite struct {
	suite.Suite
}

func (s *ConfigSeederTestSuite) loadBlueprint(yamlContent string) *schema.Blueprint {
	bp, err := schema.LoadString(yamlContent, schema.YAMLSpecFormat)
	s.Require().NoError(err, "failed to load test blueprint")
	return bp
}

func (s *ConfigSeederTestSuite) Test_sql_database_config_values_for_single_db() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  auditDb:
    type: "celerity/sqlDatabase"
    spec:
      engine: postgres
      name: audit
      schemaPath: "./schemas/audit-db.yaml"
      authMode: password
`)
	values := sqlDatabaseConfigValues(bp)
	s.Require().NotNil(values)
	s.Assert().Equal("sql-database", values["audit_host"])
	s.Assert().Equal("5432", values["audit_port"])
	s.Assert().Equal("audit", values["audit_database"])
	s.Assert().Equal("celerity", values["audit_user"])
	s.Assert().Equal("celerity", values["audit_password"])
	s.Assert().Equal("postgres", values["audit_engine"])
	s.Assert().Equal("password", values["audit_authMode"])
	s.Assert().Equal("false", values["audit_ssl"])
}

func (s *ConfigSeederTestSuite) Test_sql_database_config_values_for_multiple_dbs() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  auditDb:
    type: "celerity/sqlDatabase"
    spec:
      engine: postgres
      name: audit
  analyticsDb:
    type: "celerity/sqlDatabase"
    spec:
      engine: postgres
      name: analytics
`)
	values := sqlDatabaseConfigValues(bp)
	s.Require().NotNil(values)
	s.Assert().Equal("sql-database", values["audit_host"])
	s.Assert().Equal("audit", values["audit_database"])
	s.Assert().Equal("sql-database", values["analytics_host"])
	s.Assert().Equal("analytics", values["analytics_database"])
}

func (s *ConfigSeederTestSuite) Test_sql_database_config_values_defaults_engine() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myDb:
    type: "celerity/sqlDatabase"
    spec:
      name: mydb
`)
	values := sqlDatabaseConfigValues(bp)
	s.Require().NotNil(values)
	s.Assert().Equal("postgres", values["mydb_engine"])
	s.Assert().Equal("5432", values["mydb_port"])
}

func (s *ConfigSeederTestSuite) Test_sql_database_config_values_returns_nil_for_no_sql_dbs() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myQueue:
    type: "celerity/queue"
    spec:
      name: events
`)
	values := sqlDatabaseConfigValues(bp)
	s.Assert().Nil(values)
}

func (s *ConfigSeederTestSuite) Test_resources_config_store_env_vars() {
	envVars := ResourcesConfigStoreEnvVars()
	s.Assert().Equal("resources", envVars["CELERITY_CONFIG_RESOURCES_STORE_ID"])
}

func (s *ConfigSeederTestSuite) Test_collect_config_resources() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  appConfig:
    type: "celerity/config"
    spec:
      name: appConfig
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
`)
	resources := CollectConfigResources(bp)
	s.Require().Len(resources, 1)
	s.Assert().Equal("appConfig", resources[0].ResourceName)
	s.Assert().Equal("appConfig", resources[0].StoreName)
}

func (s *ConfigSeederTestSuite) Test_collect_config_resources_defaults_name_to_resource_name() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myConfig:
    type: "celerity/config"
    spec: {}
`)
	resources := CollectConfigResources(bp)
	s.Require().Len(resources, 1)
	s.Assert().Equal("myConfig", resources[0].StoreName)
}

func (s *ConfigSeederTestSuite) Test_collect_config_resources_nil_resources() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources: {}
`)
	s.Assert().Empty(CollectConfigResources(bp))
}

func (s *ConfigSeederTestSuite) Test_config_store_id_env_vars() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  appConfig:
    type: "celerity/config"
    spec:
      name: appConfig
`)
	envVars := ConfigStoreIDEnvVars(bp)
	s.Assert().Equal("appConfig", envVars["CELERITY_CONFIG_APPCONFIG_STORE_ID"])
	s.Assert().Equal("appConfig", envVars["CELERITY_CONFIG_APPCONFIG_NAMESPACE"])
}

func (s *ConfigSeederTestSuite) Test_config_store_id_env_vars_no_config_resources() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
`)
	s.Assert().Nil(ConfigStoreIDEnvVars(bp))
}

func (s *ConfigSeederTestSuite) Test_resource_config_values_with_cache() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  sessions:
    type: "celerity/cache"
    spec:
      name: sessions
`)
	values := ResourceConfigValues(bp)
	s.Require().NotNil(values)
	s.Assert().Equal("valkey", values["sessions_host"])
	s.Assert().Equal("6379", values["sessions_port"])
	s.Assert().Equal("false", values["sessions_tls"])
}

func (s *ConfigSeederTestSuite) Test_resource_config_values_with_datastore() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
`)
	values := ResourceConfigValues(bp)
	s.Require().NotNil(values)
	s.Assert().Equal("users", values["users"])
}

func (s *ConfigSeederTestSuite) Test_resource_config_values_with_bucket() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  assetsBucket:
    type: "celerity/bucket"
    spec:
      name: assets
`)
	values := ResourceConfigValues(bp)
	s.Assert().Equal("assets", values["assets"])
}

func (s *ConfigSeederTestSuite) Test_resource_config_values_with_queue_and_topic() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  eventsQueue:
    type: "celerity/queue"
    spec:
      name: events
  notifTopic:
    type: "celerity/topic"
    spec:
      name: notifications
`)
	values := ResourceConfigValues(bp)
	s.Assert().Equal("events", values["events"])
	s.Assert().Equal("notifications", values["notifications"])
}

func (s *ConfigSeederTestSuite) Test_resource_config_values_mysql_port() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myDb:
    type: "celerity/sqlDatabase"
    spec:
      name: mydb
      engine: mysql
`)
	values := ResourceConfigValues(bp)
	s.Assert().Equal("3306", values["mydb_port"])
	s.Assert().Equal("mysql", values["mydb_engine"])
}

func (s *ConfigSeederTestSuite) Test_resource_config_values_nil_resources() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources: {}
`)
	s.Assert().Nil(ResourceConfigValues(bp))
}

func (s *ConfigSeederTestSuite) Test_load_config_yaml() {
	dir := s.T().TempDir()
	path := dir + "/config.yaml"
	s.Require().NoError(os.WriteFile(path, []byte("api_key: abc123\nmax_retries: 3\n"), 0o644))

	values, err := LoadConfigYAML(path)
	s.Require().NoError(err)
	s.Assert().Equal("abc123", values["api_key"])
	s.Assert().Equal("3", values["max_retries"])
}

func (s *ConfigSeederTestSuite) Test_load_config_yaml_nonexistent_returns_empty() {
	values, err := LoadConfigYAML("/nonexistent/config.yaml")
	s.Require().NoError(err)
	s.Assert().Empty(values)
}

func (s *ConfigSeederTestSuite) Test_load_config_yaml_invalid_returns_error() {
	dir := s.T().TempDir()
	path := dir + "/bad.yaml"
	s.Require().NoError(os.WriteFile(path, []byte(": [invalid"), 0o644))

	_, err := LoadConfigYAML(path)
	s.Assert().Error(err)
}

func (s *ConfigSeederTestSuite) Test_load_and_merge_config_secrets_override() {
	configDir := s.T().TempDir()
	secretsDir := s.T().TempDir()

	s.Require().NoError(os.WriteFile(configDir+"/app.yaml", []byte("url: http://example.com\napi_key: default\n"), 0o644))
	s.Require().NoError(os.WriteFile(secretsDir+"/app.yaml", []byte("api_key: secret123\n"), 0o644))

	values, err := LoadAndMergeConfig(configDir, secretsDir, "app")
	s.Require().NoError(err)
	s.Assert().Equal("http://example.com", values["url"])
	s.Assert().Equal("secret123", values["api_key"])
}

func (s *ConfigSeederTestSuite) Test_load_and_merge_config_empty_dirs() {
	values, err := LoadAndMergeConfig("", "", "app")
	s.Require().NoError(err)
	s.Assert().Empty(values)
}

func TestConfigSeederTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigSeederTestSuite))
}

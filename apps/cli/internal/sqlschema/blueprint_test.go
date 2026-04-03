package sqlschema

import (
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/suite"
)

type BlueprintTestSuite struct {
	suite.Suite
}

func TestBlueprintTestSuite(t *testing.T) {
	suite.Run(t, new(BlueprintTestSuite))
}

func (s *BlueprintTestSuite) loadBlueprint(yamlContent string) *schema.Blueprint {
	bp, err := schema.LoadString(yamlContent, schema.YAMLSpecFormat)
	s.Require().NoError(err, "failed to load test blueprint")
	return bp
}

func (s *BlueprintTestSuite) Test_collect_database_resources_with_sql_database() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  mainDb:
    type: "celerity/sqlDatabase"
    spec:
      name: myapp
      engine: postgres
      schemaPath: "./db/schema.sql"
      migrationsPath: "./db/migrations"
`)
	resources := CollectDatabaseResources(bp, "/project")
	s.Require().Len(resources, 1)
	s.Assert().Equal("mainDb", resources[0].ResourceName)
	s.Assert().Equal("myapp", resources[0].Name)
	s.Assert().Equal("postgres", resources[0].Engine)
	s.Assert().Contains(resources[0].SchemaPath, "db/schema.sql")
	s.Assert().Contains(resources[0].MigrationsPath, "db/migrations")
}

func (s *BlueprintTestSuite) Test_collect_database_resources_defaults_engine_to_postgres() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  mainDb:
    type: "celerity/sqlDatabase"
    spec:
      name: myapp
`)
	resources := CollectDatabaseResources(bp, "/project")
	s.Require().Len(resources, 1)
	s.Assert().Equal("postgres", resources[0].Engine)
}

func (s *BlueprintTestSuite) Test_collect_database_resources_defaults_name_to_resource_name() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  mainDb:
    type: "celerity/sqlDatabase"
    spec: {}
`)
	resources := CollectDatabaseResources(bp, "/project")
	s.Require().Len(resources, 1)
	s.Assert().Equal("mainDb", resources[0].Name)
}

func (s *BlueprintTestSuite) Test_collect_database_resources_ignores_non_sql_resources() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
`)
	resources := CollectDatabaseResources(bp, "/project")
	s.Assert().Empty(resources)
}

func (s *BlueprintTestSuite) Test_collect_database_resources_nil_resources_returns_nil() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources: {}
`)
	resources := CollectDatabaseResources(bp, "/project")
	s.Assert().Empty(resources)
}

func (s *BlueprintTestSuite) Test_has_sql_database_resources_true() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  mainDb:
    type: "celerity/sqlDatabase"
    spec:
      name: myapp
`)
	s.Assert().True(HasSqlDatabaseResources(bp))
}

func (s *BlueprintTestSuite) Test_has_sql_database_resources_false() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
`)
	s.Assert().False(HasSqlDatabaseResources(bp))
}

func (s *BlueprintTestSuite) Test_has_sql_database_resources_nil_resources() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources: {}
`)
	s.Assert().False(HasSqlDatabaseResources(bp))
}

func (s *BlueprintTestSuite) Test_format_connection_string() {
	result := FormatConnectionString("localhost", "5432", "user", "pass", "mydb")
	s.Assert().Equal("postgres://user:pass@localhost:5432/mydb?sslmode=disable", result)
}

func (s *BlueprintTestSuite) Test_default_postgres_credentials() {
	user, password, database := DefaultPostgresCredentials()
	s.Assert().Equal("celerity", user)
	s.Assert().Equal("celerity", password)
	s.Assert().Equal("celerity", database)
}

func (s *BlueprintTestSuite) Test_resource_type() {
	s.Assert().Equal("celerity/sqlDatabase", ResourceType())
}

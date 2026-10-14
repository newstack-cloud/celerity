package seed

import (
	"sort"
	"strings"
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/suite"
)

// The keys each resource type contributes to the "resources" config namespace,
// pinned so this seeder and the AWS transformer cannot drift apart unnoticed.
//
// Both write the same namespace, read by the same SDK layers, but they are in
// separate repositories with no dependency between them, so nothing but a test
// on each side keeps them honest. The transformer's half lives in
// bluelink-transformer-celerity/transformer/resources_store_cache_test.go.
//
// This has already gone wrong once and cost a day. The transformer implemented
// the namespace for queue, topic, datastore and bucket only, while this seeder
// covered all six, so local development worked perfectly and every deployed
// handler touching a cache or a database failed at cold start on `Config
// namespace "resources" not registered`, after deploying entirely cleanly.
//
// Deployed values differ from these, and should. A real cache has a generated
// AUTH token in a secret store and the namespace carries a reference to it
// (`<name>_authTokenSecretId`), where local development has no password at all.
// What must not differ is the set of keys the SDK can rely on finding.
type ConfigKeysContractTestSuite struct {
	suite.Suite
}

func TestConfigKeysContractTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigKeysContractTestSuite))
}

func (s *ConfigKeysContractTestSuite) suffixesFor(values map[string]string, name string) []string {
	suffixes := []string{}
	for key := range values {
		if suffix, found := strings.CutPrefix(key, name); found {
			suffixes = append(suffixes, suffix)
		}
	}
	sort.Strings(suffixes)
	return suffixes
}

func (s *ConfigKeysContractTestSuite) Test_cache_keys() {
	bp := loadBlueprintFor(s, `
version: 2025-11-02
resources:
  sessions:
    type: "celerity/cache"
    spec:
      name: sessions
`)

	// There is no _authTokenSecretId because there is no secret store locally,
	// and no password on the local Valkey. The deployed side adds it.
	s.Equal(
		[]string{"_authMode", "_clusterMode", "_host", "_port", "_tls"},
		s.suffixesFor(ResourceConfigValues(bp), "sessions"),
	)
}

func (s *ConfigKeysContractTestSuite) Test_sql_database_keys() {
	bp := loadBlueprintFor(s, `
version: 2025-11-02
resources:
  auditDb:
    type: "celerity/sqlDatabase"
    spec:
      engine: postgres
      name: audit
      schemaPath: "./schemas/audit-db.yaml"
`)

	// The deployed side does not write these yet, because a deployed database
	// has no application user to hand out credentials for. See the in-VPC
	// maintenance runner design in docs/schema-commands/.
	s.Equal(
		[]string{
			"_authMode",
			"_database",
			"_engine",
			"_host",
			"_password",
			"_port",
			"_ssl",
			"_user",
		},
		s.suffixesFor(ResourceConfigValues(bp), "audit"),
	)
}

// The four simple types are a single identifier each: a queue URL, a topic ARN,
// a table name, a bucket name. Nothing to authenticate with, which is why they
// were the four the deploy path managed to implement.
func (s *ConfigKeysContractTestSuite) Test_simple_types_are_a_single_key() {
	bp := loadBlueprintFor(s, `
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
  tasks:
    type: "celerity/queue"
    spec:
      name: taskQueue
`)

	values := ResourceConfigValues(bp)

	s.Equal([]string{""}, s.suffixesFor(values, "users"))
	s.Equal([]string{""}, s.suffixesFor(values, "taskQueue"))
}

// Separate from the sibling suite's parsing helper so neither suite depends on
// the other's fixtures.
func loadBlueprintFor(s *ConfigKeysContractTestSuite, yamlContent string) *schema.Blueprint {
	bp, err := schema.LoadString(yamlContent, schema.YAMLSpecFormat)
	s.Require().NoError(err, "failed to load test blueprint")
	return bp
}

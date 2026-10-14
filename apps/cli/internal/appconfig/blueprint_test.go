package appconfig

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

func (s *BlueprintTestSuite) Test_collect_store_info_secrets_manager() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  appSecrets:
    type: "celerity/config"
    spec:
      name: my-app-secrets
`)
	stores := CollectStoreInfo(bp)
	s.Require().Len(stores, 1)
	s.Equal("appSecrets", stores[0].ResourceName)
	s.Equal("my-app-secrets", stores[0].StoreName)
	s.Equal(StoreKindSecretsManager, stores[0].StoreKind)
	s.Empty(stores[0].PlaintextKeys)
}

func (s *BlueprintTestSuite) Test_collect_store_info_parameter_store_with_plaintext() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  appConfig:
    type: "celerity/config"
    spec:
      name: my-app-config
      plaintext:
        - FETCH_TIMEOUT
        - LOG_LEVEL
        - POOL_MAX
`)
	stores := CollectStoreInfo(bp)
	s.Require().Len(stores, 1)
	s.Equal("appConfig", stores[0].ResourceName)
	s.Equal("my-app-config", stores[0].StoreName)
	s.Equal(StoreKindParameterStore, stores[0].StoreKind)
	s.Len(stores[0].PlaintextKeys, 3)
	s.True(stores[0].PlaintextKeys["FETCH_TIMEOUT"])
	s.True(stores[0].PlaintextKeys["LOG_LEVEL"])
	s.True(stores[0].PlaintextKeys["POOL_MAX"])
}

func (s *BlueprintTestSuite) Test_collect_store_info_defaults_name_to_resource_name() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myConfig:
    type: "celerity/config"
    spec: {}
`)
	stores := CollectStoreInfo(bp)
	s.Require().Len(stores, 1)
	s.Equal("myConfig", stores[0].StoreName)
	s.Equal(StoreKindSecretsManager, stores[0].StoreKind)
}

func (s *BlueprintTestSuite) Test_collect_store_info_multiple_stores() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  appConfig:
    type: "celerity/config"
    spec:
      name: app-config
      plaintext:
        - TIMEOUT
  appSecrets:
    type: "celerity/config"
    spec:
      name: app-secrets
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
`)
	stores := CollectStoreInfo(bp)
	s.Require().Len(stores, 2)
}

func (s *BlueprintTestSuite) Test_collect_store_info_nil_resources() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources: {}
`)
	s.Empty(CollectStoreInfo(bp))
}

func (s *BlueprintTestSuite) Test_collect_store_info_no_config_resources() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
`)
	s.Empty(CollectStoreInfo(bp))
}

func (s *BlueprintTestSuite) Test_find_store_info_by_spec_name() {
	stores := []StoreInfo{
		{ResourceName: "appConfig", StoreName: "my-app-config", StoreKind: StoreKindParameterStore},
		{ResourceName: "appSecrets", StoreName: "my-app-secrets", StoreKind: StoreKindSecretsManager},
	}
	found := FindStoreInfo(stores, "my-app-secrets")
	s.Require().NotNil(found)
	s.Equal("appSecrets", found.ResourceName)
}

func (s *BlueprintTestSuite) Test_find_store_info_by_resource_name() {
	stores := []StoreInfo{
		{ResourceName: "appConfig", StoreName: "my-app-config", StoreKind: StoreKindParameterStore},
	}
	found := FindStoreInfo(stores, "appConfig")
	s.Require().NotNil(found)
	s.Equal("my-app-config", found.StoreName)
}

func (s *BlueprintTestSuite) Test_find_store_info_spec_name_takes_precedence() {
	stores := []StoreInfo{
		{ResourceName: "cfg", StoreName: "cfg", StoreKind: StoreKindSecretsManager},
	}
	// "cfg" matches both the spec name and the resource name, and the spec name
	// match comes first.
	found := FindStoreInfo(stores, "cfg")
	s.Require().NotNil(found)
	s.Equal("cfg", found.StoreName)
}

func (s *BlueprintTestSuite) Test_find_store_info_not_found() {
	stores := []StoreInfo{
		{ResourceName: "appConfig", StoreName: "my-app-config"},
	}
	s.Nil(FindStoreInfo(stores, "nonexistent"))
}

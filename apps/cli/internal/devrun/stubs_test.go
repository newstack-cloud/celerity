package devrun

import (
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/compose"
	"github.com/newstack-cloud/celerity/apps/cli/internal/seed"
	"github.com/stretchr/testify/suite"
)

type StubsTestSuite struct {
	suite.Suite
}

func TestStubsTestSuite(t *testing.T) {
	suite.Run(t, new(StubsTestSuite))
}

func (s *StubsTestSuite) Test_declared_namespace_wins() {
	svc := compose.StubServiceInfo{
		Name:            "enrichment-service",
		ConfigKey:       "ENRICHMENT_SERVICE_BASE_URL",
		ConfigNamespace: "appConfig",
	}
	configs := []seed.ConfigResourceInfo{
		{ResourceName: "appConfig", StoreName: "appConfig"},
		{ResourceName: "appConfig2", StoreName: "appConfig2"},
	}

	namespace, err := resolveStubConfigNamespace(svc, configs)
	s.Require().NoError(err)
	s.Assert().Equal("appConfig", namespace)
}

func (s *StubsTestSuite) Test_single_config_resource_is_used_by_default() {
	svc := compose.StubServiceInfo{Name: "enrichment-service", ConfigKey: "BASE_URL"}
	configs := []seed.ConfigResourceInfo{{ResourceName: "appConfig", StoreName: "app-config"}}

	namespace, err := resolveStubConfigNamespace(svc, configs)
	s.Require().NoError(err)
	s.Assert().Equal("app-config", namespace)
}

func (s *StubsTestSuite) Test_several_config_resources_without_a_declared_namespace_is_an_error() {
	// Guessing here silently seeds the URL into a store the application never
	// reads, which only shows up later as requests going to the real endpoint.
	svc := compose.StubServiceInfo{Name: "enrichment-service", ConfigKey: "ENRICHMENT_SERVICE_BASE_URL"}
	configs := []seed.ConfigResourceInfo{
		{ResourceName: "appConfig", StoreName: "appConfig"},
		{ResourceName: "appConfig2", StoreName: "appConfig2"},
	}

	_, err := resolveStubConfigNamespace(svc, configs)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "configNamespace")
	s.Assert().Contains(err.Error(), "appConfig2", "the error must name the candidate stores")
}

func (s *StubsTestSuite) Test_no_config_resources_skips_seeding() {
	svc := compose.StubServiceInfo{Name: "enrichment-service", ConfigKey: "BASE_URL"}

	namespace, err := resolveStubConfigNamespace(svc, nil)
	s.Require().NoError(err)
	s.Assert().Empty(namespace)
}

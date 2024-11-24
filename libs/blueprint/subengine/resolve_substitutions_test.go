package subengine

import (
	"context"
	"os"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/providerhelpers"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
)

type SubstitutionResolverTestSuite struct {
	specFixtureFiles   map[string]string
	specFixtureSchemas map[string]*schema.Blueprint
	resourceRegistry   resourcehelpers.Registry
	funcRegistry       provider.FunctionRegistry
	dataSourceRegistry provider.DataSourceRegistry
	stateContainer     state.Container
	resourceCache      *core.Cache[*provider.ResolvedResource]
	suite.Suite
}

func (s *SubstitutionResolverTestSuite) SetupSuite() {
	s.specFixtureFiles = map[string]string{
		"resolve-in-resource": "__testdata/sub-resolver/resolve-in-resource-blueprint.yml",
	}
	s.specFixtureSchemas = make(map[string]*schema.Blueprint)

	for name, filePath := range s.specFixtureFiles {
		specBytes, err := os.ReadFile(filePath)
		if err != nil {
			s.FailNow(err.Error())
		}
		blueprintStr := string(specBytes)
		blueprint, err := schema.LoadString(blueprintStr, schema.YAMLSpecFormat)
		if err != nil {
			s.FailNow(err.Error())
		}
		s.specFixtureSchemas[name] = blueprint

	}
}

func (s *SubstitutionResolverTestSuite) SetupTest() {
	s.stateContainer = internal.NewMemoryStateContainer()
	providers := map[string]provider.Provider{
		"aws": newTestAWSProvider(),
		"core": providerhelpers.NewCoreProvider(
			s.stateContainer,
			core.BlueprintInstanceIDFromContext,
			os.Getwd,
			core.SystemClock{},
		),
	}
	s.funcRegistry = provider.NewFunctionRegistry(providers)
	s.resourceRegistry = resourcehelpers.NewRegistry(
		providers,
		map[string]transform.SpecTransformer{},
	)
	s.dataSourceRegistry = provider.NewDataSourceRegistry(
		providers,
	)
	s.resourceCache = core.NewCache[*provider.ResolvedResource]()
}

func (s *SubstitutionResolverTestSuite) Test_resolves_substitutions_in_resource_for_change_staging() {
	// See: __testdata/sub-resolver/resolve-in-resource-blueprint.yml
	blueprint := s.specFixtureSchemas["resolve-in-resource"]
	spec := internal.NewBlueprintSpecMock(blueprint)
	environment := "production-env"
	enableOrderTableTrigger := true
	region := "us-west-2"
	deployOrdersTableToRegions := "[\"us-west-2\",\"us-east-1\"]"
	blueprintVars := map[string]*core.ScalarValue{
		"environment": {
			StringValue: &environment,
		},
		"region": {
			StringValue: &region,
		},
		"deployOrdersTableToRegions": {
			StringValue: &deployOrdersTableToRegions,
		},
		"enableOrderTableTrigger": {
			BoolValue: &enableOrderTableTrigger,
		},
	}
	params := internal.NewParams(
		map[string]map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
		blueprintVars,
	)
	subResolver := NewDefaultSubstitutionResolver(
		s.funcRegistry,
		s.resourceRegistry,
		s.dataSourceRegistry,
		s.stateContainer,
		s.resourceCache,
		spec,
		params,
	)

	result, err := subResolver.ResolveInResource(
		context.TODO(),
		"ordersTable",
		blueprint.Resources.Values["ordersTable"],
		&ResolveResourceTargetInfo{
			ResolveFor: ResolveForChangeStaging,
		},
	)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	err = cupaloy.Snapshot(result)
	s.Require().NoError(err)
}

func TestSubstitutionResolverTestSuite(t *testing.T) {
	suite.Run(t, new(SubstitutionResolverTestSuite))
}

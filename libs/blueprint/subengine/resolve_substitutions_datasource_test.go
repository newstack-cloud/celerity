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

type SubstitutionDataSourceResolverTestSuite struct {
	specFixtureFiles   map[string]string
	specFixtureSchemas map[string]*schema.Blueprint
	resourceRegistry   resourcehelpers.Registry
	funcRegistry       provider.FunctionRegistry
	dataSourceRegistry provider.DataSourceRegistry
	stateContainer     state.Container
	resourceCache      *core.Cache[*provider.ResolvedResource]
	suite.Suite
}

const (
	resolveInDataSourceFixtureName = "resolve-in-datasource"
)

func (s *SubstitutionDataSourceResolverTestSuite) SetupSuite() {
	s.specFixtureFiles = map[string]string{
		resolveInDataSourceFixtureName: "__testdata/sub-resolver/resolve-in-datasource-blueprint.yml",
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

func (s *SubstitutionDataSourceResolverTestSuite) SetupTest() {
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

func (s *SubstitutionDataSourceResolverTestSuite) Test_resolves_substitutions_in_datasource_for_change_staging() {
	blueprint := s.specFixtureSchemas[resolveInDataSourceFixtureName]
	spec := internal.NewBlueprintSpecMock(blueprint)
	params := resolveInDataSourceTestParams()
	subResolver := NewDefaultSubstitutionResolver(
		s.funcRegistry,
		s.resourceRegistry,
		s.dataSourceRegistry,
		s.stateContainer,
		s.resourceCache,
		spec,
		params,
	)

	result, err := subResolver.ResolveInDataSource(
		context.TODO(),
		"network",
		blueprint.DataSources.Values["network"],
		&ResolveDataSourceTargetInfo{
			ResolveFor: ResolveForChangeStaging,
		},
	)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	err = cupaloy.Snapshot(result)
	s.Require().NoError(err)
}

func (s *SubstitutionDataSourceResolverTestSuite) Test_resolves_substitutions_in_datasource_for_deployment() {
	blueprint := s.specFixtureSchemas[resolveInDataSourceFixtureName]
	spec := internal.NewBlueprintSpecMock(blueprint)
	params := resolveInDataSourceTestParams()
	subResolver := NewDefaultSubstitutionResolver(
		s.funcRegistry,
		s.resourceRegistry,
		s.dataSourceRegistry,
		s.stateContainer,
		s.resourceCache,
		spec,
		params,
	)
	// ordersTable.spec.id is used in the resource and should be resolved using the resource
	// state.
	err := s.stateContainer.SaveInstance(context.Background(), state.InstanceState{
		InstanceID: testInstanceID,
	})
	s.Require().NoError(err)

	resourceID := "test-orders-table-309428320"
	err = s.stateContainer.SaveResource(
		context.Background(),
		testInstanceID,
		state.ResourceState{
			ResourceID:   resourceID,
			ResourceName: "ordersTable",
			ResourceSpecData: &core.MappingNode{
				Fields: map[string]*core.MappingNode{
					"id": {
						Literal: &core.ScalarValue{
							StringValue: &resourceID,
						},
					},
				},
			},
		},
	)
	s.Require().NoError(err)

	// Make sure the current instance ID can be retrieved from the context when fetching
	// state from the state container to resolve the resource spec id reference.
	ctx := context.WithValue(
		context.Background(),
		core.BlueprintInstanceIDKey,
		testInstanceID,
	)

	// The resource must be resolved before the data source can be resolved.
	// During change staging, the blueprint container will make sure that resources
	// are ordered correctly so that by the time any resource is referenced, it has
	// already been resolved.
	s.resourceCache.Set("ordersTable", &provider.ResolvedResource{})

	result, err := subResolver.ResolveInDataSource(
		ctx,
		"network",
		blueprint.DataSources.Values["network"],
		&ResolveDataSourceTargetInfo{
			ResolveFor:        ResolveForDeployment,
			PartiallyResolved: partiallyResolvedDataSource(),
		},
	)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	err = cupaloy.Snapshot(result)
	s.Require().NoError(err)
}

func partiallyResolvedDataSource() *provider.ResolvedDataSource {
	displayName := "Networking"
	filterField := "tags"
	subnetIdsDescription := "The IDs of the subnets."
	vpcDescription := "The ID of the VPC.\n"
	vpcField := "vpcId"
	dataSourceDescription := "Networking resources for the application."
	return &provider.ResolvedDataSource{
		Type: &schema.DataSourceTypeWrapper{
			Value: "aws/vpc",
		},
		DataSourceMetadata: &provider.ResolvedDataSourceMetadata{
			DisplayName: &core.MappingNode{
				Literal: &core.ScalarValue{
					StringValue: &displayName,
				},
			},
		},
		Filter: &provider.ResolvedDataSourceFilter{
			Field: &core.ScalarValue{
				StringValue: &filterField,
			},
			Operator: &schema.DataSourceFilterOperatorWrapper{
				Value: schema.DataSourceFilterOperatorNotContains,
			},
		},
		Exports: map[string]*provider.ResolvedDataSourceFieldExport{
			"subnetIds": {
				Type: &schema.DataSourceFieldTypeWrapper{
					Value: schema.DataSourceFieldTypeArray,
				},
				Description: &core.MappingNode{
					Literal: &core.ScalarValue{
						StringValue: &subnetIdsDescription,
					},
				},
			},
			"vpc": {
				Type: &schema.DataSourceFieldTypeWrapper{
					Value: schema.DataSourceFieldTypeString,
				},
				AliasFor: &core.ScalarValue{
					StringValue: &vpcField,
				},
				Description: &core.MappingNode{
					Literal: &core.ScalarValue{
						StringValue: &vpcDescription,
					},
				},
			},
		},
		Description: &core.MappingNode{
			Literal: &core.ScalarValue{
				StringValue: &dataSourceDescription,
			},
		},
	}
}

func resolveInDataSourceTestParams() *internal.Params {
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
	return internal.NewParams(
		map[string]map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
		blueprintVars,
	)
}

func TestSubstitutionDataSourceResolverTestSuite(t *testing.T) {
	suite.Run(t, new(SubstitutionDataSourceResolverTestSuite))
}

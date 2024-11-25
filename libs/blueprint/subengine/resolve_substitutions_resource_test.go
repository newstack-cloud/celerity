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

type SubstitutionResourceResolverTestSuite struct {
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
	resolveInResourceFixtureName = "resolve-in-resource"
	testInstanceID               = "cb826a32-1052-4fde-aa6e-d449b9f50066"
)

func (s *SubstitutionResourceResolverTestSuite) SetupSuite() {
	s.specFixtureFiles = map[string]string{
		resolveInResourceFixtureName: "__testdata/sub-resolver/resolve-in-resource-blueprint.yml",
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

func (s *SubstitutionResourceResolverTestSuite) SetupTest() {
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

func (s *SubstitutionResourceResolverTestSuite) Test_resolves_substitutions_in_resource_for_change_staging() {
	blueprint := s.specFixtureSchemas[resolveInResourceFixtureName]
	spec := internal.NewBlueprintSpecMock(blueprint)
	params := resolveInResourceTestParams()
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

func (s *SubstitutionResourceResolverTestSuite) Test_resolves_substitutions_in_resource_for_deployment() {
	blueprint := s.specFixtureSchemas[resolveInResourceFixtureName]
	spec := internal.NewBlueprintSpecMock(blueprint)
	params := resolveInResourceTestParams()
	subResolver := NewDefaultSubstitutionResolver(
		s.funcRegistry,
		s.resourceRegistry,
		s.dataSourceRegistry,
		s.stateContainer,
		s.resourceCache,
		spec,
		params,
	)
	// coreInfra.region is used in the resource and should be resolved using the child blueprint
	// state.
	err := s.stateContainer.SaveInstance(context.Background(), state.InstanceState{
		InstanceID: testInstanceID,
	})
	s.Require().NoError(err)

	childBlueprintRegion := "eu-west-1"
	err = s.stateContainer.SaveChild(
		context.Background(),
		testInstanceID,
		"coreInfra",
		state.InstanceState{
			Exports: map[string]*core.MappingNode{
				"region": {
					Literal: &core.ScalarValue{
						StringValue: &childBlueprintRegion,
					},
				},
			},
		},
	)
	s.Require().NoError(err)

	// Make sure the current instance ID can be retrieved from the context when fetching
	// state from the state container to resolve the child blueprint export reference.
	ctx := context.WithValue(
		context.Background(),
		core.BlueprintInstanceIDKey,
		testInstanceID,
	)
	result, err := subResolver.ResolveInResource(
		ctx,
		"ordersTable",
		blueprint.Resources.Values["ordersTable"],
		&ResolveResourceTargetInfo{
			ResolveFor:        ResolveForDeployment,
			PartiallyResolved: partiallyResolvedResource(),
		},
	)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	err = cupaloy.Snapshot(result)
	s.Require().NoError(err)
}

func partiallyResolvedResource() *provider.ResolvedResource {
	description := "Table that stores orders for an application."
	displayName := "production-env Orders Table"
	trigger := true
	x := 100
	y := 200
	condition1 := true
	condition2 := true
	condition3 := false
	tableName := "production-Orders"
	return &provider.ResolvedResource{
		Type: &schema.ResourceTypeWrapper{
			Value: "aws/dynamodb/table",
		},
		Description: &core.MappingNode{
			Literal: &core.ScalarValue{
				StringValue: &description,
			},
		},
		Metadata: &provider.ResolvedResourceMetadata{
			DisplayName: &core.MappingNode{
				Literal: &core.ScalarValue{
					StringValue: &displayName,
				},
			},
			Annotations: &core.MappingNode{
				Fields: map[string]*core.MappingNode{
					"aws.dynamodb.trigger": {
						Literal: &core.ScalarValue{
							BoolValue: &trigger,
						},
					},
				},
			},
			Labels: &schema.StringMap{
				Values: map[string]string{
					"app": "orders",
				},
			},
			Custom: &core.MappingNode{
				Fields: map[string]*core.MappingNode{
					"label": {
						Literal: &core.ScalarValue{
							StringValue: &displayName,
						},
					},
					"x": {
						Literal: &core.ScalarValue{
							IntValue: &x,
						},
					},
					"y": {
						Literal: &core.ScalarValue{
							IntValue: &y,
						},
					},
				},
			},
		},
		Condition: &provider.ResolvedResourceCondition{
			And: []*provider.ResolvedResourceCondition{
				{
					StringValue: &core.MappingNode{
						Literal: &core.ScalarValue{
							BoolValue: &condition1,
						},
					},
				},
				{
					Or: []*provider.ResolvedResourceCondition{
						{
							StringValue: &core.MappingNode{
								Literal: &core.ScalarValue{
									BoolValue: &condition2,
								},
							},
						},
						{
							Not: &provider.ResolvedResourceCondition{
								StringValue: &core.MappingNode{
									Literal: &core.ScalarValue{
										BoolValue: &condition3,
									},
								},
							},
						},
					},
				},
			},
		},
		LinkSelector: &schema.LinkSelector{
			ByLabel: &schema.StringMap{
				Values: map[string]string{
					"app": "orders",
				},
			},
		},
		Spec: &core.MappingNode{
			Fields: map[string]*core.MappingNode{
				"region": (*core.MappingNode)(nil),
				"tableName": {
					Literal: &core.ScalarValue{
						StringValue: &tableName,
					},
				},
			},
		},
	}
}

func resolveInResourceTestParams() *internal.Params {
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

func TestSubstitutionResourceResolverTestSuite(t *testing.T) {
	suite.Run(t, new(SubstitutionResourceResolverTestSuite))
}

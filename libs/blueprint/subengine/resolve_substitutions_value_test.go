package subengine

import (
	"context"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
)

type SubstitutionValueResolverTestSuite struct {
	SubResolverTestContainer
	suite.Suite
}

const (
	resolveInValueFixtureName = "resolve-in-value"
)

func (s *SubstitutionValueResolverTestSuite) SetupSuite() {
	s.populateSpecFixtureSchemas(
		map[string]string{
			resolveInValueFixtureName: "__testdata/sub-resolver/resolve-in-value-blueprint.yml",
		},
		&s.Suite,
	)
}

func (s *SubstitutionValueResolverTestSuite) SetupTest() {
	s.populateDependencies()
}

func (s *SubstitutionValueResolverTestSuite) Test_resolves_substitutions_in_value_for_change_staging() {
	blueprint := s.specFixtureSchemas[resolveInValueFixtureName]
	spec := internal.NewBlueprintSpecMock(blueprint)
	params := resolveInValueTestParams()
	subResolver := NewDefaultSubstitutionResolver(
		&Registries{
			FuncRegistry:       s.funcRegistry,
			ResourceRegistry:   s.resourceRegistry,
			DataSourceRegistry: s.dataSourceRegistry,
		},
		s.stateContainer,
		s.resourceCache,
		s.resourceTemplateInputElemCache,
		s.childExportFieldCache,
		spec,
		params,
	)

	result, err := subResolver.ResolveInValue(
		context.TODO(),
		"deployOrdersTableToRegions",
		blueprint.Values.Values["deployOrdersTableToRegions"],
		&ResolveValueTargetInfo{
			ResolveFor: ResolveForChangeStaging,
		},
	)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	err = cupaloy.Snapshot(result)
	s.Require().NoError(err)
}

func resolveInValueTestParams() core.BlueprintParams {
	environment := "production-env"
	enableOrderTableTrigger := true
	region := "us-west-2"
	deployOrdersTableToRegions := "[\"us-west-2\",\"us-east-1\"]"
	relatedInfo := "[{\"id\":\"test-info-1\"},{\"id\":\"test-info-2\"}]"
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
		"relatedInfo": {
			StringValue: &relatedInfo,
		},
	}
	return core.NewDefaultParams(
		map[string]map[string]*core.ScalarValue{},
		map[string]map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
		blueprintVars,
	)
}

func TestSubstitutionValueResolverTestSuite(t *testing.T) {
	suite.Run(t, new(SubstitutionValueResolverTestSuite))
}

package subengine

import (
	"context"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
)

type SubstitutionExportResolverTestSuite struct {
	SubResolverTestContainer
	suite.Suite
}

const (
	resolveInExportFixtureName = "resolve-in-export"
)

func (s *SubstitutionExportResolverTestSuite) SetupSuite() {
	s.populateSpecFixtureSchemas(
		map[string]string{
			resolveInExportFixtureName: "__testdata/sub-resolver/resolve-in-export-blueprint.yml",
		},
		&s.Suite,
	)
}

func (s *SubstitutionExportResolverTestSuite) SetupTest() {
	s.populateDependencies()
}

func (s *SubstitutionExportResolverTestSuite) Test_resolves_substitutions_in_export_for_change_staging() {
	blueprint := s.specFixtureSchemas[resolveInExportFixtureName]
	spec := internal.NewBlueprintSpecMock(blueprint)
	params := resolveInExportTestParams()
	subResolver := NewDefaultSubstitutionResolver(
		s.funcRegistry,
		s.resourceRegistry,
		s.dataSourceRegistry,
		s.stateContainer,
		s.resourceCache,
		spec,
		params,
	)

	result, err := subResolver.ResolveInExport(
		context.TODO(),
		"environment",
		blueprint.Exports.Values["environment"],
		&ResolveExportTargetInfo{
			ResolveFor: ResolveForChangeStaging,
		},
	)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	err = cupaloy.Snapshot(result)
	s.Require().NoError(err)
}

func resolveInExportTestParams() *internal.Params {
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

func TestSubstitutionExportResolverTestSuite(t *testing.T) {
	suite.Run(t, new(SubstitutionExportResolverTestSuite))
}

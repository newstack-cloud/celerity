package container

import (
	"context"
	"os"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/errors"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/providerhelpers"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
)

type ExpandResourceTemplatesTestSuite struct {
	specFixtureContainers          map[string]BlueprintContainer
	stateContainer                 state.Container
	funcRegistry                   provider.FunctionRegistry
	resourceRegistry               resourcehelpers.Registry
	dataSourceRegistry             provider.DataSourceRegistry
	resourceChangeStager           ResourceChangeStager
	providers                      map[string]provider.Provider
	resourceCache                  *core.Cache[*provider.ResolvedResource]
	resourceTemplateInputElemCache *core.Cache[[]*core.MappingNode]
	suite.Suite
}

const (
	expandedOneToManyLinkFixtureName  = "expanded-one-to-many-link"
	expandedManyToOneLinkFixtureName  = "expanded-many-to-one-link"
	expandedManyToManyLinkFixtureName = "expanded-many-to-many-link"
	expandedFailureFixtureName        = "expanded-failure"
)

func (s *ExpandResourceTemplatesTestSuite) SetupSuite() {
	inputFiles := map[string]string{
		expandedOneToManyLinkFixtureName:  "__testdata/template-expansion/expanded-1-blueprint.yml",
		expandedManyToOneLinkFixtureName:  "__testdata/template-expansion/expanded-2-blueprint.yml",
		expandedManyToManyLinkFixtureName: "__testdata/template-expansion/expanded-3-blueprint.yml",
		expandedFailureFixtureName:        "__testdata/template-expansion/expanded-fail-blueprint.yml",
	}
	s.specFixtureContainers = make(map[string]BlueprintContainer)

	s.providers = map[string]provider.Provider{
		"aws": newTestAWSProvider(),
		"core": providerhelpers.NewCoreProvider(
			s.stateContainer,
			core.BlueprintInstanceIDFromContext,
			os.Getwd,
			core.SystemClock{},
		),
	}
	s.stateContainer = internal.NewMemoryStateContainer()
	loader := NewDefaultLoader(
		s.providers,
		map[string]transform.SpecTransformer{},
		s.stateContainer,
		s.resourceChangeStager,
		newFSChildResolver(),
		validation.NewRefChainCollector,
	)
	for name, filePath := range inputFiles {
		specBytes, err := os.ReadFile(filePath)
		if err != nil {
			s.FailNow(err.Error())
		}
		blueprintStr := string(specBytes)
		params := expandResourceTemplatesTestParams()
		bpContainer, err := loader.LoadString(context.TODO(), blueprintStr, schema.YAMLSpecFormat, params)
		if err != nil {
			s.FailNow(err.Error())
		}
		s.specFixtureContainers[name] = bpContainer
	}
}

func (s *ExpandResourceTemplatesTestSuite) SetupTest() {
	s.stateContainer = internal.NewMemoryStateContainer()
	s.resourceChangeStager = NewDefaultResourceChangeStager()
	s.funcRegistry = provider.NewFunctionRegistry(s.providers)
	s.resourceRegistry = resourcehelpers.NewRegistry(
		s.providers,
		map[string]transform.SpecTransformer{},
	)
	s.dataSourceRegistry = provider.NewDataSourceRegistry(
		s.providers,
	)
	s.resourceCache = core.NewCache[*provider.ResolvedResource]()
	s.resourceTemplateInputElemCache = core.NewCache[[]*core.MappingNode]()
}

func (s *ExpandResourceTemplatesTestSuite) Test_expands_resource_template_with_one_to_many_link_relationship() {
	container := s.specFixtureContainers[expandedOneToManyLinkFixtureName]
	params := expandResourceTemplatesTestParams()
	subResolver := subengine.NewDefaultSubstitutionResolver(
		&subengine.Registries{
			FuncRegistry:       s.funcRegistry,
			ResourceRegistry:   s.resourceRegistry,
			DataSourceRegistry: s.dataSourceRegistry,
		},
		s.stateContainer,
		s.resourceCache,
		s.resourceTemplateInputElemCache,
		container.BlueprintSpec(),
		params,
	)

	ctx := context.TODO()
	linkChains, err := container.SpecLinkInfo().Links(ctx)
	s.Require().NoError(err)

	result, err := ExpandResourceTemplates(
		ctx,
		container.BlueprintSpec().Schema(),
		subResolver,
		linkChains,
		s.resourceTemplateInputElemCache,
	)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	err = cupaloy.Snapshot(result)
	s.Require().NoError(err)
}

func (s *ExpandResourceTemplatesTestSuite) Test_expands_resource_template_with_many_to_one_link_relationship() {
	container := s.specFixtureContainers[expandedManyToOneLinkFixtureName]
	params := expandResourceTemplatesTestParams()
	subResolver := subengine.NewDefaultSubstitutionResolver(
		&subengine.Registries{
			FuncRegistry:       s.funcRegistry,
			ResourceRegistry:   s.resourceRegistry,
			DataSourceRegistry: s.dataSourceRegistry,
		},
		s.stateContainer,
		s.resourceCache,
		s.resourceTemplateInputElemCache,
		container.BlueprintSpec(),
		params,
	)

	ctx := context.TODO()
	linkChains, err := container.SpecLinkInfo().Links(ctx)
	s.Require().NoError(err)

	result, err := ExpandResourceTemplates(
		ctx,
		container.BlueprintSpec().Schema(),
		subResolver,
		linkChains,
		s.resourceTemplateInputElemCache,
	)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	err = cupaloy.Snapshot(result)
	s.Require().NoError(err)
}

func (s *ExpandResourceTemplatesTestSuite) Test_expands_resource_template_with_many_to_many_link_relationship() {
	container := s.specFixtureContainers[expandedManyToManyLinkFixtureName]
	params := expandResourceTemplatesTestParams()
	subResolver := subengine.NewDefaultSubstitutionResolver(
		&subengine.Registries{
			FuncRegistry:       s.funcRegistry,
			ResourceRegistry:   s.resourceRegistry,
			DataSourceRegistry: s.dataSourceRegistry,
		},
		s.stateContainer,
		s.resourceCache,
		s.resourceTemplateInputElemCache,
		container.BlueprintSpec(),
		params,
	)

	ctx := context.TODO()
	linkChains, err := container.SpecLinkInfo().Links(ctx)
	s.Require().NoError(err)

	result, err := ExpandResourceTemplates(
		ctx,
		container.BlueprintSpec().Schema(),
		subResolver,
		linkChains,
		s.resourceTemplateInputElemCache,
	)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	err = cupaloy.Snapshot(result)
	s.Require().NoError(err)
}

func (s *ExpandResourceTemplatesTestSuite) Test_fails_to_expand_when_link_relationship_between_templates_has_length_mismatch() {
	container := s.specFixtureContainers[expandedFailureFixtureName]
	params := expandResourceTemplatesTestParams()
	subResolver := subengine.NewDefaultSubstitutionResolver(
		&subengine.Registries{
			FuncRegistry:       s.funcRegistry,
			ResourceRegistry:   s.resourceRegistry,
			DataSourceRegistry: s.dataSourceRegistry,
		},
		s.stateContainer,
		s.resourceCache,
		s.resourceTemplateInputElemCache,
		container.BlueprintSpec(),
		params,
	)

	ctx := context.TODO()
	linkChains, err := container.SpecLinkInfo().Links(ctx)
	s.Require().NoError(err)

	_, err = ExpandResourceTemplates(
		ctx,
		container.BlueprintSpec().Schema(),
		subResolver,
		linkChains,
		s.resourceTemplateInputElemCache,
	)
	s.Require().Error(err)
	runError, isRunError := err.(*errors.RunError)
	s.Assert().True(isRunError)
	s.Assert().Equal(ErrorReasonCodeResourceTemplateLinkLengthMismatch, runError.ReasonCode)
	s.Assert().Equal(
		"run error: resource template function has a link "+
			"to resource template ordersTable with a different input length,"+
			" links between resource templates can only be made when the "+
			"resolved items list from the `each` property of both templates"+
			" is of the same length",
		runError.Error(),
	)
}

func expandResourceTemplatesTestParams() *internal.Params {
	environment := "production-env"
	tablesConfig := "[{\"name\":\"orders-1\"},{\"name\":\"orders-2\"},{\"name\":\"orders-3\"}]"
	functionsConfig := "[{\"handler\":\"ordersFunction-1\"},{\"handler\":\"ordersFunction-2\"},{\"handler\":\"ordersFunction-3\"}]"
	otherFunctionsConfig := "[{\"handler\":\"otherFunction-1\"}]"
	blueprintVars := map[string]*core.ScalarValue{
		"environment": {
			StringValue: &environment,
		},
		"tablesConfig": {
			StringValue: &tablesConfig,
		},
		"functionsConfig": {
			StringValue: &functionsConfig,
		},
		"otherFunctionsConfig": {
			StringValue: &otherFunctionsConfig,
		},
	}
	return internal.NewParams(
		map[string]map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
		blueprintVars,
	)
}

func TestExpandResourceTemplatesTestSuite(t *testing.T) {
	suite.Run(t, new(ExpandResourceTemplatesTestSuite))
}

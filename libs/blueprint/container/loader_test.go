package container

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/providerhelpers"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
)

type LoaderTestSuite struct {
	specFixtures                 map[string]string
	specFixtureFiles             map[string]string
	specFixtureSchemas           map[string]*schema.Blueprint
	loader                       Loader
	loaderDefaultCore            Loader
	loaderValidateAfterTransform Loader
	suite.Suite
}

const (
	validServerlessBlueprintName = "valid-serverless"
)

func (s *LoaderTestSuite) SetupSuite() {
	s.specFixtures = make(map[string]string)
	s.specFixtureFiles = map[string]string{
		"valid":                       "__testdata/loader/valid-blueprint.yml",
		"invalid-yaml":                "__testdata/loader/invalid-yaml-blueprint.yml",
		"invalid-schema":              "__testdata/loader/invalid-schema-blueprint.yml",
		"unsupported-var-type":        "__testdata/loader/unsupported-var-type-blueprint.yml",
		validServerlessBlueprintName:  "__testdata/loader/valid-serverless-blueprint.yml",
		"missing-transform":           "__testdata/loader/missing-transform-blueprint.yml",
		"cyclic-ref":                  "__testdata/loader/cyclic-ref-blueprint.yml",
		"cyclic-ref-2":                "__testdata/loader/cyclic-ref-2-blueprint.yml",
		"cyclic-ref-3":                "__testdata/loader/cyclic-ref-3-blueprint.yml",
		"cyclic-ref-4":                "__testdata/loader/cyclic-ref-4-blueprint.yml",
		"cyclic-ref-5":                "__testdata/loader/cyclic-ref-5-blueprint.yml",
		"cyclic-ref-6":                "__testdata/loader/cyclic-ref-6-blueprint.yml",
		"invalid-resource-each-dep-1": "__testdata/loader/invalid-resource-each-dep-1-blueprint.yml",
		"invalid-resource-each-dep-2": "__testdata/loader/invalid-resource-each-dep-2-blueprint.yml",
	}
	s.specFixtureSchemas = make(map[string]*schema.Blueprint)

	for name, filePath := range s.specFixtureFiles {
		specBytes, err := os.ReadFile(filePath)
		if err != nil {
			s.FailNow(err.Error())
		}
		blueprintStr := string(specBytes)
		s.specFixtures[name] = blueprintStr
		if strings.HasPrefix(name, "valid") {
			blueprint, err := schema.LoadString(blueprintStr, schema.YAMLSpecFormat)
			if err != nil {
				s.FailNow(err.Error())
			}
			s.specFixtureSchemas[name] = blueprint
		}
	}

	stateContainer := internal.NewMemoryStateContainer()
	resourceChangeStager := NewDefaultResourceChangeStager()
	providers := map[string]provider.Provider{
		"aws": newTestAWSProvider(),
		"core": providerhelpers.NewCoreProvider(
			stateContainer,
			core.BlueprintInstanceIDFromContext,
			os.Getwd,
			core.SystemClock{},
		),
	}
	specTransformers := map[string]transform.SpecTransformer{
		"serverless-2024": &internal.ServerlessTransformer{},
	}
	s.loader = NewDefaultLoader(
		providers,
		specTransformers,
		stateContainer,
		resourceChangeStager,
		newFSChildResolver(),
		validation.NewRefChainCollector,
		WithLoaderTransformSpec(true),
	)
	s.loaderValidateAfterTransform = NewDefaultLoader(
		providers,
		specTransformers,
		stateContainer,
		resourceChangeStager,
		newFSChildResolver(),
		validation.NewRefChainCollector,
		WithLoaderTransformSpec(true),
		WithLoaderValidateAfterTransform(true),
	)
	providersWithoutCore := map[string]provider.Provider{
		"aws": newTestAWSProvider(),
	}
	s.loaderDefaultCore = NewDefaultLoader(
		providersWithoutCore,
		specTransformers,
		stateContainer,
		resourceChangeStager,
		newFSChildResolver(),
		validation.NewRefChainCollector,
	)
}

func (s *LoaderTestSuite) Test_loads_container_from_input_spec_file_without_any_issues() {
	container, err := s.loader.Load(context.TODO(), s.specFixtureFiles["valid"], createParams())
	s.Require().NoError(err)
	s.Assert().NotNil(container)
}

func (s *LoaderTestSuite) Test_loads_container_from_input_spec_file_using_default_core_provider() {
	container, err := s.loaderDefaultCore.Load(context.TODO(), s.specFixtureFiles["valid"], createParams())
	s.Require().NoError(err)
	s.Assert().NotNil(container)
}

func (s *LoaderTestSuite) Test_loads_container_from_input_spec_string_without_any_issues() {
	container, err := s.loader.LoadString(context.TODO(), s.specFixtures["valid"], schema.YAMLSpecFormat, createParams())
	s.Require().NoError(err)
	s.Assert().NotNil(container)
}

func (s *LoaderTestSuite) Test_validates_spec_from_input_spec_file_without_any_issues() {
	validationRes, err := s.loader.Validate(context.TODO(), s.specFixtureFiles["valid"], createParams())
	s.Require().NoError(err)
	s.Assert().NotNil(validationRes)
}

func (s *LoaderTestSuite) Test_validates_spec_from_input_spec_string_without_any_issues() {
	validationRes, err := s.loader.ValidateString(context.TODO(), s.specFixtures["valid"], schema.YAMLSpecFormat, createParams())
	s.Require().NoError(err)
	s.Assert().NotNil(validationRes)
}

func (s *LoaderTestSuite) Test_loads_container_from_input_schema_without_any_issues() {
	container, err := s.loader.LoadFromSchema(context.TODO(), s.specFixtureSchemas["valid"], createParams())
	s.Require().NoError(err)
	s.Assert().NotNil(container)
}

func (s *LoaderTestSuite) Test_loads_and_transforms_input_blueprint_without_any_issues() {
	container, err := s.loader.Load(context.TODO(), s.specFixtureFiles[validServerlessBlueprintName], createParams())
	s.Require().NoError(err)
	s.Assert().NotNil(container)
}

func (s *LoaderTestSuite) Test_loads_and_transforms_input_blueprint_validating_after_transform() {
	container, err := s.loaderValidateAfterTransform.Load(context.TODO(), s.specFixtureFiles[validServerlessBlueprintName], createParams())
	s.Require().NoError(err)
	s.Assert().NotNil(container)
}

func (s *LoaderTestSuite) Test_validates_spec_from_input_schema_without_any_issues() {
	validationRes, err := s.loader.ValidateFromSchema(context.TODO(), s.specFixtureSchemas["valid"], createParams())
	s.Require().NoError(err)
	s.Assert().NotNil(validationRes)
}

func (s *LoaderTestSuite) Test_reports_expected_error_when_the_provided_spec_is_invalid() {
	// This is for when the spec is invalid JSON/YAML, as test coverage for specific formats
	// is handled by the schema package, we just need to ensure that the error is reported
	// for either format.
	_, err := s.loader.Load(context.TODO(), s.specFixtureFiles["invalid-yaml"], createParams())
	s.Require().Error(err)
}

func (s *LoaderTestSuite) Test_reports_expected_error_when_the_provided_spec_fails_schema_specific_validation() {
	// This is for when the spec is valid JSON/YAML, but fails validation against the schema.
	_, err := s.loader.Load(context.TODO(), s.specFixtureFiles["invalid-schema"], createParams())
	s.Require().Error(err)
}

func (s *LoaderTestSuite) Test_reports_expected_error_when_the_provided_spec_contains_unsupported_variable_types() {
	_, err := s.loader.Load(context.TODO(), s.specFixtureFiles["unsupported-var-type"], createParams())
	s.Require().Error(err)
}

func (s *LoaderTestSuite) Test_reports_expected_error_when_transform_is_missing() {
	_, err := s.loader.Load(context.TODO(), s.specFixtureFiles["missing-transform"], createParams())
	s.Require().Error(err)
}

func (s *LoaderTestSuite) Test_reports_error_for_blueprint_with_cyclic_references() {
	_, err := s.loader.Load(context.TODO(), s.specFixtureFiles["cyclic-ref"], createParams())
	s.Require().Error(err)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	s.Assert().True(isLoadErr)
	s.Assert().Equal(
		validation.ErrorReasonCodeReferenceCycle,
		loadErr.ReasonCode,
	)
}

func (s *LoaderTestSuite) Test_reports_error_for_blueprint_with_cyclic_link_and_reference() {
	_, err := s.loader.Load(context.TODO(), s.specFixtureFiles["cyclic-ref-2"], createParams())
	s.Require().Error(err)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	s.Assert().True(isLoadErr)
	s.Assert().Equal(
		validation.ErrorReasonCodeReferenceCycle,
		loadErr.ReasonCode,
	)
}

func (s *LoaderTestSuite) Test_reports_error_for_blueprint_with_hard_cyclic_link() {
	_, err := s.loader.Load(context.TODO(), s.specFixtureFiles["cyclic-ref-3"], createParams())
	s.Require().Error(err)
	linkErr, isLinkErr := err.(*links.LinkError)
	s.Assert().True(isLinkErr)
	s.Assert().Equal(
		links.LinkErrorReasonCodeCircularLinks,
		linkErr.ReasonCode,
	)
}

func (s *LoaderTestSuite) Test_reports_error_for_blueprint_with_indirect_cyclic_link() {
	_, err := s.loader.Load(context.TODO(), s.specFixtureFiles["cyclic-ref-4"], createParams())
	s.Require().Error(err)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	s.Assert().True(isLoadErr)
	s.Assert().Equal(
		validation.ErrorReasonCodeReferenceCycle,
		loadErr.ReasonCode,
	)
}

func (s *LoaderTestSuite) Test_reports_error_for_blueprint_with_indirect_cyclic_link_with_explicit_dependency() {
	_, err := s.loader.Load(context.TODO(), s.specFixtureFiles["cyclic-ref-5"], createParams())
	s.Require().Error(err)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	s.Assert().True(isLoadErr)
	s.Assert().Equal(
		validation.ErrorReasonCodeReferenceCycle,
		loadErr.ReasonCode,
	)
}

func (s *LoaderTestSuite) Test_reports_error_for_blueprint_with_indirect_cyclic_link_via_link_func_arg() {
	_, err := s.loader.Load(context.TODO(), s.specFixtureFiles["cyclic-ref-6"], createParams())
	s.Require().Error(err)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	s.Assert().True(isLoadErr)
	s.Assert().Equal(
		validation.ErrorReasonCodeReferenceCycle,
		loadErr.ReasonCode,
	)
}

func (s *LoaderTestSuite) Test_reports_error_for_blueprint_with_invalid_resource_each_dependency_1() {
	_, err := s.loader.Load(context.TODO(), s.specFixtureFiles["invalid-resource-each-dep-1"], createParams())
	s.Require().Error(err)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	s.Assert().True(isLoadErr)
	s.Assert().Equal(
		validation.ErrorReasonCodeEachResourceDependency,
		loadErr.ReasonCode,
	)
}

func (s *LoaderTestSuite) Test_reports_error_for_blueprint_with_invalid_resource_each_dependency_2() {
	_, err := s.loader.Load(context.TODO(), s.specFixtureFiles["invalid-resource-each-dep-2"], createParams())
	s.Require().Error(err)
	loadErr, isLoadErr := internal.UnpackLoadError(err)
	s.Assert().True(isLoadErr)
	s.Assert().Equal(
		validation.ErrorReasonCodeEachChildDependency,
		loadErr.ReasonCode,
	)
}

func TestLoaderTestSuite(t *testing.T) {
	suite.Run(t, new(LoaderTestSuite))
}

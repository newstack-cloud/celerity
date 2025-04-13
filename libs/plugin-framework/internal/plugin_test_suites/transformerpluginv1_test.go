package plugintestsuites

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testtransformer"
	"github.com/two-hundred/celerity/libs/plugin-framework/internal/testutils"
	"github.com/two-hundred/celerity/libs/plugin-framework/pluginservicev1"
	"github.com/two-hundred/celerity/libs/plugin-framework/transformerserverv1"
)

type TransformerPluginV1Suite struct {
	pluginService        pluginservicev1.ServiceClient
	transformer          transform.SpecTransformer
	transformerWrongHost transform.SpecTransformer
	failingTransformer   transform.SpecTransformer
	funcRegistry         provider.FunctionRegistry

	closePluginService      func()
	closeTransformer        func()
	closeFailingTransformer func()
	suite.Suite
}

func (s *TransformerPluginV1Suite) SetupSuite() {
	transformers := map[string]transform.SpecTransformer{}
	pluginManager := pluginservicev1.NewManager(
		map[pluginservicev1.PluginType]string{
			pluginservicev1.PluginType_PLUGIN_TYPE_PROVIDER:    "1.0",
			pluginservicev1.PluginType_PLUGIN_TYPE_TRANSFORMER: "1.0",
		},
		s.createPluginInstance,
	)
	s.funcRegistry = provider.NewFunctionRegistry(
		map[string]provider.Provider{},
	)
	pluginService, closePluginService := testutils.StartPluginServiceServer(
		testHostID,
		pluginManager,
		s.funcRegistry,
		resourcehelpers.NewRegistry(
			map[string]provider.Provider{},
			transformers,
			/* stabilisationPollingInterval */ 1*time.Millisecond,
			core.NewDefaultParams(
				map[string]map[string]*core.ScalarValue{},
				map[string]map[string]*core.ScalarValue{},
				map[string]*core.ScalarValue{},
				map[string]*core.ScalarValue{},
			),
		),
	)
	s.pluginService = pluginService
	s.closePluginService = closePluginService

	transformerClient, closeTransformer := testtransformer.StartPluginServer(
		pluginService,
		/* failingPlugin */ false,
	)
	s.closeTransformer = closeTransformer
	s.transformer = transformerserverv1.WrapTransformerClient(transformerClient, testHostID)
	transformers["celerityTransform"] = s.transformer

	failingTransformerClient, closeFailingTransformer := testtransformer.StartPluginServer(
		pluginService,
		/* failingPlugin */ true,
	)
	s.closeFailingTransformer = closeFailingTransformer
	s.failingTransformer = transformerserverv1.WrapTransformerClient(
		failingTransformerClient,
		testHostID,
	)

	s.transformerWrongHost = transformerserverv1.WrapTransformerClient(
		transformerClient,
		testWrongHostID,
	)
}

func (s *TransformerPluginV1Suite) Test_get_transform_name() {
	transformName, err := s.transformer.GetTransformName(context.Background())
	s.Require().NoError(err)
	s.Require().Equal("celerity-2025-04-01", transformName)
}

func (s *TransformerPluginV1Suite) Test_get_transform_name_fails_for_unexpected_host() {
	_, err := s.transformerWrongHost.GetTransformName(context.Background())
	testutils.AssertInvalidHost(
		err,
		errorsv1.PluginActionTransformerGetTransformName,
		testWrongHostID,
		&s.Suite,
	)
}

func (s *TransformerPluginV1Suite) Test_get_transform_name_reports_expected_error_for_failure() {
	_, err := s.failingTransformer.GetTransformName(context.Background())
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "internal error occurred retrieving transform name")
}

func (s *TransformerPluginV1Suite) Test_get_config_definition() {
	configDefinition, err := s.transformer.ConfigDefinition(context.Background())
	s.Require().NoError(err)
	testutils.AssertConfigDefinitionEquals(
		testtransformer.TestTransformerConfigDefinition(),
		configDefinition,
		&s.Suite,
	)
}

func (s *TransformerPluginV1Suite) Test_get_config_definition_fails_for_unexpected_host() {
	_, err := s.transformerWrongHost.ConfigDefinition(context.Background())
	testutils.AssertInvalidHost(
		err,
		errorsv1.PluginActionTransformerGetConfigDefinition,
		testWrongHostID,
		&s.Suite,
	)
}

func (s *TransformerPluginV1Suite) Test_get_config_definition_reports_expected_error_for_failure() {
	_, err := s.failingTransformer.ConfigDefinition(context.Background())
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "internal error occurred retrieving config definition")
}

func (s *TransformerPluginV1Suite) createPluginInstance(
	info *pluginservicev1.PluginInstanceInfo,
) (any, func(), error) {
	// This is required for the manager that backs the plugin service that allows
	// the plugin to register itself with the host service.
	// For the transformer v1 plugin test suite, we are not testing the management
	// of plugins, the transformer plugin to be tested is instantiated as a part of
	// the test suite setup.
	return nil, nil, nil
}

func (s *TransformerPluginV1Suite) TearDownSuite() {
	s.closeTransformer()
	s.closeFailingTransformer()
	// We must close the plugin service after the transformer plugin
	// so it can deregister itself.
	s.closePluginService()
}

func TestTransformerPluginV1Suite(t *testing.T) {
	suite.Run(t, new(TransformerPluginV1Suite))
}

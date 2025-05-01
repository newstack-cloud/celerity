package pluginservicev1

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"
)

const (
	testHostID = "test-host"
)

type ManagerSuite struct {
	manager Manager
	suite.Suite
}

func (s *ManagerSuite) SetupTest() {
	s.manager = NewManager(
		map[PluginType]string{
			PluginType_PLUGIN_TYPE_PROVIDER:    "1.8",
			PluginType_PLUGIN_TYPE_TRANSFORMER: "1.8",
		},
		s.pluginFactory,
		testHostID,
	)
}

func (s *ManagerSuite) Test_fails_to_register_plugin_with_unsupported_type() {
	err := s.manager.RegisterPlugin(
		&PluginInstanceInfo{
			PluginType: PluginType_PLUGIN_TYPE_NONE,
			ID:         "test-plugin",
		},
	)
	s.Require().Error(err)
	s.Assert().Equal("plugin type 0 is not supported", err.Error())
}

func (s *ManagerSuite) Test_fails_to_register_plugin_with_incompatible_protocol_versions_different_major() {
	err := s.manager.RegisterPlugin(
		&PluginInstanceInfo{
			PluginType:       PluginType_PLUGIN_TYPE_PROVIDER,
			ID:               "test-plugin",
			ProtocolVersions: []string{"2.2", "3.0"},
		},
	)
	s.Require().Error(err)
	s.Assert().Equal("plugin protocol versions \"2.2, 3.0\" are not supported, expected <=1.8", err.Error())
}

func (s *ManagerSuite) Test_fails_to_register_plugin_with_incompatible_protocol_versions_future_minor() {
	err := s.manager.RegisterPlugin(
		&PluginInstanceInfo{
			PluginType:       PluginType_PLUGIN_TYPE_PROVIDER,
			ID:               "test-plugin",
			ProtocolVersions: []string{"1.12", "2.2", "3.0"},
		},
	)
	s.Require().Error(err)
	s.Assert().Equal("plugin protocol versions \"1.12, 2.2, 3.0\" are not supported, expected <=1.8", err.Error())
}

func (s *ManagerSuite) Test_fails_to_register_plugin_that_has_already_been_registered() {
	pluginInstanceInfo := &PluginInstanceInfo{
		PluginType:       PluginType_PLUGIN_TYPE_PROVIDER,
		ID:               "test-plugin",
		ProtocolVersions: []string{"1.2", "2.2", "3.0"},
	}
	err := s.manager.RegisterPlugin(pluginInstanceInfo)
	s.Require().NoError(err)

	err = s.manager.RegisterPlugin(pluginInstanceInfo)
	s.Require().Error(err)
	s.Assert().Equal("plugin test-plugin is already registered", err.Error())
}

func (s *ManagerSuite) Test_successfully_registers_plugin_with_compatible_protocol_version() {
	err := s.manager.RegisterPlugin(
		&PluginInstanceInfo{
			PluginType:       PluginType_PLUGIN_TYPE_PROVIDER,
			ID:               "test-plugin",
			ProtocolVersions: []string{"1.2", "2.2", "3.0"},
		},
	)
	s.Assert().NoError(err)
}

func (s *ManagerSuite) Test_successfully_retrieves_plugin_instance() {
	pluginInstanceInfo := &PluginInstanceInfo{
		PluginType:       PluginType_PLUGIN_TYPE_PROVIDER,
		ID:               "test-plugin",
		ProtocolVersions: []string{"1.2", "2.2", "3.0"},
	}
	err := s.manager.RegisterPlugin(pluginInstanceInfo)
	s.Require().NoError(err)

	pluginInstance := s.manager.GetPlugin(
		PluginType_PLUGIN_TYPE_PROVIDER,
		"test-plugin",
	)
	s.Require().NotNil(pluginInstance)
	s.Assert().Equal(pluginInstanceInfo, pluginInstance.Info)
}

func (s *ManagerSuite) Test_successfully_retrieves_plugin_metadata() {
	pluginInstanceInfo := &PluginInstanceInfo{
		PluginType:       PluginType_PLUGIN_TYPE_PROVIDER,
		ID:               "test-plugin-2",
		ProtocolVersions: []string{"1.3", "2.2", "3.0"},
		Metadata: &PluginMetadata{
			PluginVersion: "1.2.5",
			DisplayName:   "Test Plugin 2",
		},
	}
	err := s.manager.RegisterPlugin(pluginInstanceInfo)
	s.Require().NoError(err)

	pluginMetadata := s.manager.GetPluginMetadata(
		PluginType_PLUGIN_TYPE_PROVIDER,
		"test-plugin-2",
	)
	s.Require().NotNil(pluginMetadata)
	s.Assert().Equal(
		pluginInstanceInfo.Metadata.PluginVersion,
		pluginMetadata.PluginVersion,
	)
	s.Assert().Equal(
		pluginInstanceInfo.Metadata.DisplayName,
		pluginMetadata.DisplayName,
	)
	s.Assert().Equal(
		pluginInstanceInfo.ProtocolVersions,
		pluginMetadata.ProtocolVersions,
	)
}

func (s *ManagerSuite) Test_successfully_retrieves_all_plugins_of_given_type() {
	providerPlugin := &PluginInstanceInfo{
		PluginType:       PluginType_PLUGIN_TYPE_PROVIDER,
		ID:               "test-plugin-3",
		ProtocolVersions: []string{"1.5", "2.2", "3.0"},
		Metadata: &PluginMetadata{
			PluginVersion: "1.2.5",
			DisplayName:   "Test Plugin 3",
		},
	}
	err := s.manager.RegisterPlugin(providerPlugin)
	s.Require().NoError(err)

	transformerPlugin := &PluginInstanceInfo{
		PluginType:       PluginType_PLUGIN_TYPE_TRANSFORMER,
		ID:               "test-plugin-4",
		ProtocolVersions: []string{"1.2", "2.2", "3.0"},
		Metadata: &PluginMetadata{
			PluginVersion: "1.2.5",
			DisplayName:   "Test Plugin 4",
		},
	}
	err = s.manager.RegisterPlugin(transformerPlugin)
	s.Require().NoError(err)

	providerPlugins := s.manager.GetPlugins(
		PluginType_PLUGIN_TYPE_PROVIDER,
	)
	s.assertPluginsList([]*PluginInstanceInfo{providerPlugin}, providerPlugins)

	transformerPlugins := s.manager.GetPlugins(
		PluginType_PLUGIN_TYPE_TRANSFORMER,
	)
	s.assertPluginsList([]*PluginInstanceInfo{transformerPlugin}, transformerPlugins)
}

func (s *ManagerSuite) pluginFactory(
	_ *PluginInstanceInfo,
	_ string,
) (any, func(), error) {
	return nil, func() {}, nil
}

func (s *ManagerSuite) assertPluginsList(
	expected []*PluginInstanceInfo,
	actual []*PluginInstance,
) {
	s.Require().Len(actual, len(expected))
	// Sort lists to ensure consistent ordering for comparison.
	slices.SortFunc(
		expected,
		comparePluginInstanceInfo,
	)
	slices.SortFunc(
		actual,
		comparePluginInstance,
	)
	for i, plugin := range actual {
		s.Assert().Equal(expected[i].ID, plugin.Info.ID)
		s.Assert().Equal(
			expected[i].PluginType,
			plugin.Info.PluginType,
		)
		s.Assert().Equal(
			expected[i].Metadata.PluginVersion,
			plugin.Info.Metadata.PluginVersion,
		)
		s.Assert().Equal(
			expected[i].Metadata.DisplayName,
			plugin.Info.Metadata.DisplayName,
		)
		s.Assert().Equal(
			expected[i].ProtocolVersions,
			plugin.Info.ProtocolVersions,
		)
	}
}

func comparePluginInstanceInfo(
	a, b *PluginInstanceInfo,
) int {
	if a.ID < b.ID {
		return -1
	}

	if a.ID > b.ID {
		return 1
	}

	return 0
}

func comparePluginInstance(
	a, b *PluginInstance,
) int {
	if a.Info.ID < b.Info.ID {
		return -1
	}

	if a.Info.ID > b.Info.ID {
		return 1
	}

	return 0
}

func TestManagerSuite(t *testing.T) {
	suite.Run(t, new(ManagerSuite))
}

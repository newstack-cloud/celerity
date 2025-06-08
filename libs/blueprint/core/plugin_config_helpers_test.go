package core

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type PluginConfigHelpersTestSuite struct {
	suite.Suite
}

func (s *PluginConfigHelpersTestSuite) Test_plugin_config_get_all_with_prefix() {
	config := PluginConfig{
		"aws.config.regionKMSKeys.0.field1": ScalarFromString("value1"),
		"aws.config.regionKMSKeys.0.field2": ScalarFromString("value1.2"),
		"aws.config.regionKMSKeys.1.field1": ScalarFromString("value2"),
		"aws.config.regionKMSKeys.1.field2": ScalarFromString("value2.2"),
		"aws.config.regionKMSKeys.2.field1": ScalarFromString("value3"),
		"aws.config.regionKMSKeys.2.field2": ScalarFromString("value3.2"),
		"aws.config.otherKey":               ScalarFromString("value"),
	}
	subset := config.GetAllWithPrefix("aws.config.regionKMSKeys")
	s.Assert().Len(subset, 6)
	s.Assert().Equal("value1", StringValueFromScalar(subset["aws.config.regionKMSKeys.0.field1"]))
	s.Assert().Equal("value1.2", StringValueFromScalar(subset["aws.config.regionKMSKeys.0.field2"]))
	s.Assert().Equal("value2", StringValueFromScalar(subset["aws.config.regionKMSKeys.1.field1"]))
	s.Assert().Equal("value2.2", StringValueFromScalar(subset["aws.config.regionKMSKeys.1.field2"]))
	s.Assert().Equal("value3", StringValueFromScalar(subset["aws.config.regionKMSKeys.2.field1"]))
	s.Assert().Equal("value3.2", StringValueFromScalar(subset["aws.config.regionKMSKeys.2.field2"]))
}

func (s *PluginConfigHelpersTestSuite) Test_plugin_config_returns_input_for_empty_prefix() {
	config := PluginConfig{
		"aws.config.regionKMSKeys.0": ScalarFromString("value1"),
		"aws.config.regionKMSKeys.1": ScalarFromString("value2"),
		"aws.config.regionKMSKeys.2": ScalarFromString("value3"),
		"aws.config.otherKey":        ScalarFromString("value"),
	}
	subset := config.GetAllWithPrefix("")
	s.Assert().Equal(config, subset)
}

func (s *PluginConfigHelpersTestSuite) Test_plugin_config_get_all_with_slice_prefix_simple_structure() {
	config := PluginConfig{
		"aws.config.regionKMSKeys.0": ScalarFromString("value1"),
		"aws.config.regionKMSKeys.1": ScalarFromString("value2"),
		"aws.config.regionKMSKeys.2": ScalarFromString("value3"),
		"aws.config.otherKey":        ScalarFromString("value"),
	}
	subset, keys := config.GetAllWithSlicePrefix("aws.config.regionKMSKeys")
	s.Assert().Len(subset, 3)
	s.Assert().Equal("value1", StringValueFromScalar(subset["aws.config.regionKMSKeys.0"]))
	s.Assert().Equal("value2", StringValueFromScalar(subset["aws.config.regionKMSKeys.1"]))
	s.Assert().Equal("value3", StringValueFromScalar(subset["aws.config.regionKMSKeys.2"]))
	s.Assert().Equal(
		[]string{
			"aws.config.regionKMSKeys.0",
			"aws.config.regionKMSKeys.1",
			"aws.config.regionKMSKeys.2",
		},
		keys,
	)
}

func (s *PluginConfigHelpersTestSuite) Test_plugin_config_get_all_with_slice_prefix_complex_structure() {
	config := PluginConfig{
		"aws.config.regionKMSKeys.0.field1": ScalarFromString("value1"),
		"aws.config.regionKMSKeys.0.field2": ScalarFromString("value1.2"),
		"aws.config.regionKMSKeys.1.field1": ScalarFromString("value2"),
		"aws.config.regionKMSKeys.1.field2": ScalarFromString("value2.2"),
		"aws.config.regionKMSKeys.2.field1": ScalarFromString("value3"),
		"aws.config.regionKMSKeys.2.field2": ScalarFromString("value3.2"),
		"aws.config.otherKey":               ScalarFromString("value"),
	}
	subset, keys := config.GetAllWithSlicePrefix("aws.config.regionKMSKeys")
	s.Assert().Len(subset, 6)
	s.Assert().Equal("value1", StringValueFromScalar(subset["aws.config.regionKMSKeys.0.field1"]))
	s.Assert().Equal("value1.2", StringValueFromScalar(subset["aws.config.regionKMSKeys.0.field2"]))
	s.Assert().Equal("value2", StringValueFromScalar(subset["aws.config.regionKMSKeys.1.field1"]))
	s.Assert().Equal("value2.2", StringValueFromScalar(subset["aws.config.regionKMSKeys.1.field2"]))
	s.Assert().Equal("value3", StringValueFromScalar(subset["aws.config.regionKMSKeys.2.field1"]))
	s.Assert().Equal("value3.2", StringValueFromScalar(subset["aws.config.regionKMSKeys.2.field2"]))
	s.Assert().Len(keys, 6)
	// Assert each key is present, not the order as the fields inside an array
	// prefix are not guaranteed to be in order.
	s.Assert().Contains(keys, "aws.config.regionKMSKeys.0.field1")
	s.Assert().Contains(keys, "aws.config.regionKMSKeys.0.field2")
	s.Assert().Contains(keys, "aws.config.regionKMSKeys.1.field1")
	s.Assert().Contains(keys, "aws.config.regionKMSKeys.1.field2")
	s.Assert().Contains(keys, "aws.config.regionKMSKeys.2.field2")
	s.Assert().Contains(keys, "aws.config.regionKMSKeys.2.field1")
}

func (s *PluginConfigHelpersTestSuite) Test_plugin_config_get_all_with_slice_prefix_empty_prefix() {
	config := PluginConfig{
		"aws.config.regionKMSKeys.0": ScalarFromString("value1"),
		"aws.config.regionKMSKeys.1": ScalarFromString("value2"),
		"aws.config.regionKMSKeys.2": ScalarFromString("value3"),
		"aws.config.otherKey":        ScalarFromString("value"),
	}
	subset, keys := config.GetAllWithSlicePrefix("")
	s.Assert().Equal(config, subset)
	s.Assert().Nil(keys)
}

func (s *PluginConfigHelpersTestSuite) Test_plugin_config_get_all_with_map_prefix() {
	config := PluginConfig{
		"aws.config.regionKMSKeys.key1": ScalarFromString("value1"),
		"aws.config.regionKMSKeys.key2": ScalarFromString("value2"),
		"aws.config.regionKMSKeys.key3": ScalarFromString("value3"),
		"aws.config.otherKey":           ScalarFromString("value"),
	}
	subset := config.GetAllWithMapPrefix("aws.config.regionKMSKeys")
	s.Assert().Len(subset, 3)
	s.Assert().Equal("value1", StringValueFromScalar(subset["aws.config.regionKMSKeys.key1"]))
	s.Assert().Equal("value2", StringValueFromScalar(subset["aws.config.regionKMSKeys.key2"]))
	s.Assert().Equal("value3", StringValueFromScalar(subset["aws.config.regionKMSKeys.key3"]))
}

func (s *PluginConfigHelpersTestSuite) Test_plugin_config_get_all_with_map_prefix_empty_prefix() {
	config := PluginConfig{
		"aws.config.regionKMSKeys.key1": ScalarFromString("value1"),
		"aws.config.regionKMSKeys.key2": ScalarFromString("value2"),
		"aws.config.regionKMSKeys.key3": ScalarFromString("value3"),
		"aws.config.otherKey":           ScalarFromString("value"),
	}
	subset := config.GetAllWithMapPrefix("")
	s.Assert().Equal(config, subset)
}

func (s *PluginConfigHelpersTestSuite) Test_plugin_config_slice_from_prefix() {
	config := PluginConfig{
		"aws.config.regionKMSKeys.0": ScalarFromString("value1"),
		"aws.config.regionKMSKeys.1": ScalarFromString("value2"),
		"aws.config.regionKMSKeys.2": ScalarFromString("value3"),
		"aws.config.otherKey":        ScalarFromString("value"),
	}
	slice := config.SliceFromPrefix("aws.config.regionKMSKeys")
	s.Assert().Len(slice, 3)
	s.Assert().Equal("value1", StringValueFromScalar(slice[0]))
	s.Assert().Equal("value2", StringValueFromScalar(slice[1]))
	s.Assert().Equal("value3", StringValueFromScalar(slice[2]))
}

func (s *PluginConfigHelpersTestSuite) Test_plugin_config_slice_from_prefix_empty_prefix() {
	config := PluginConfig{
		"aws.config.regionKMSKeys.0": ScalarFromString("value1"),
		"aws.config.regionKMSKeys.1": ScalarFromString("value2"),
		"aws.config.regionKMSKeys.2": ScalarFromString("value3"),
		"aws.config.otherKey":        ScalarFromString("value"),
	}
	slice := config.SliceFromPrefix("")
	s.Assert().Len(slice, 0)
}

func (s *PluginConfigHelpersTestSuite) Test_plugin_config_slice_from_prefix_invalid_prefix() {
	config := PluginConfig{
		"aws.config.regionKMSKeys.0": ScalarFromString("value1"),
		"aws.config.regionKMSKeys.1": ScalarFromString("value2"),
		"aws.config.regionKMSKeys.2": ScalarFromString("value3"),
		"aws.config.otherKey":        ScalarFromString("value"),
	}
	slice := config.SliceFromPrefix("invalid.prefix")
	s.Assert().Len(slice, 0)
}

func (s *PluginConfigHelpersTestSuite) Test_plugin_config_map_from_prefix() {
	config := PluginConfig{
		"aws.config.regionKMSKeys.key1": ScalarFromString("value1"),
		"aws.config.regionKMSKeys.key2": ScalarFromString("value2"),
		"aws.config.regionKMSKeys.key3": ScalarFromString("value3"),
		"aws.config.otherKey":           ScalarFromString("value"),
	}
	mapping := config.MapFromPrefix("aws.config.regionKMSKeys")
	s.Assert().Len(mapping, 3)
	s.Assert().Equal("value1", StringValueFromScalar(mapping["key1"]))
	s.Assert().Equal("value2", StringValueFromScalar(mapping["key2"]))
	s.Assert().Equal("value3", StringValueFromScalar(mapping["key3"]))
}

func (s *PluginConfigHelpersTestSuite) Test_plugin_config_map_from_prefix_empty_prefix_returns_empty_map() {
	config := PluginConfig{
		"aws.config.regionKMSKeys.key1": ScalarFromString("value1"),
		"aws.config.regionKMSKeys.key2": ScalarFromString("value2"),
		"aws.config.regionKMSKeys.key3": ScalarFromString("value3"),
		"aws.config.otherKey":           ScalarFromString("value"),
	}
	mapping := config.MapFromPrefix("")
	s.Assert().Empty(mapping)
}

func TestPluginConfigHelpersTestSuite(t *testing.T) {
	suite.Run(t, new(PluginConfigHelpersTestSuite))
}

package core

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type UtilsTestSuite struct {
	suite.Suite
}

func (s *UtilsTestSuite) Test_converts_string_slice_to_mapping_node() {
	strings := []string{"value one", "value two", "value three"}
	mappingNode := MappingNodeFromStringSlice(strings)

	s.Assert().Equal(
		&MappingNode{
			Items: []*MappingNode{
				MappingNodeFromString("value one"),
				MappingNodeFromString("value two"),
				MappingNodeFromString("value three"),
			},
		},
		mappingNode,
	)
}

func (s *UtilsTestSuite) Test_converts_int_slice_to_mapping_node() {
	ints := []int64{504, 1024, 2048}
	mappingNode := MappingNodeFromIntSlice(ints)

	s.Assert().Equal(
		&MappingNode{
			Items: []*MappingNode{
				MappingNodeFromInt(504),
				MappingNodeFromInt(1024),
				MappingNodeFromInt(2048),
			},
		},
		mappingNode,
	)
}

func (s *UtilsTestSuite) Test_converts_float_slice_to_mapping_node() {
	floats := []float64{1.23, 4.56, 7.89}
	mappingNode := MappingNodeFromFloatSlice(floats)

	s.Assert().Equal(
		&MappingNode{
			Items: []*MappingNode{
				MappingNodeFromFloat(1.23),
				MappingNodeFromFloat(4.56),
				MappingNodeFromFloat(7.89),
			},
		},
		mappingNode,
	)
}

func (s *UtilsTestSuite) Test_converts_bool_to_mapping_node() {
	bools := []bool{true, false, true}
	mappingNode := MappingNodeFromBoolSlice(bools)

	s.Assert().Equal(
		&MappingNode{
			Items: []*MappingNode{
				MappingNodeFromBool(true),
				MappingNodeFromBool(false),
				MappingNodeFromBool(true),
			},
		},
		mappingNode,
	)
}

func (s *UtilsTestSuite) Test_converts_string_map_to_mapping_node() {
	stringMap := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	mappingNode := MappingNodeFromStringMap(stringMap)

	s.Assert().Equal(
		&MappingNode{
			Fields: map[string]*MappingNode{
				"key1": MappingNodeFromString("value1"),
				"key2": MappingNodeFromString("value2"),
				"key3": MappingNodeFromString("value3"),
			},
		},
		mappingNode,
	)
}

func (s *UtilsTestSuite) Test_converts_int_map_to_mapping_node() {
	intMap := map[string]int64{
		"key1": 100,
		"key2": 200,
		"key3": 300,
	}
	mappingNode := MappingNodeFromIntMap(intMap)

	s.Assert().Equal(
		&MappingNode{
			Fields: map[string]*MappingNode{
				"key1": MappingNodeFromInt(100),
				"key2": MappingNodeFromInt(200),
				"key3": MappingNodeFromInt(300),
			},
		},
		mappingNode,
	)
}

func (s *UtilsTestSuite) Test_converts_float_map_to_mapping_node() {
	floatMap := map[string]float64{
		"key1": 13.14039,
		"key2": 2.2102,
		"key3": 3.3,
	}
	mappingNode := MappingNodeFromFloatMap(floatMap)

	s.Assert().Equal(
		&MappingNode{
			Fields: map[string]*MappingNode{
				"key1": MappingNodeFromFloat(13.14039),
				"key2": MappingNodeFromFloat(2.2102),
				"key3": MappingNodeFromFloat(3.3),
			},
		},
		mappingNode,
	)
}

func (s *UtilsTestSuite) Test_converts_bool_map_to_mapping_node() {
	boolMap := map[string]bool{
		"key1": true,
		"key2": false,
		"key3": true,
	}
	mappingNode := MappingNodeFromBoolMap(boolMap)

	s.Assert().Equal(
		&MappingNode{
			Fields: map[string]*MappingNode{
				"key1": MappingNodeFromBool(true),
				"key2": MappingNodeFromBool(false),
				"key3": MappingNodeFromBool(true),
			},
		},
		mappingNode,
	)
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

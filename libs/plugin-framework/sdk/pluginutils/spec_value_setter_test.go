package pluginutils

import (
	"testing"

	"github.com/newstack-cloud/celerity/libs/blueprint/core"
	"github.com/stretchr/testify/suite"
)

type SpecValueSetterSuite struct {
	suite.Suite
}

type testTarget struct {
	FieldValue string
}

func (s *SpecValueSetterSuite) Test_sets_value_on_target_when_value_is_not_nil() {
	setter := SpecValueSetter[*testTarget]{
		PathInSpec: "$.testField",
		SetValueFunc: func(value *core.MappingNode, target *testTarget) {
			target.FieldValue = core.StringValue(value)
		},
	}

	specData := &core.MappingNode{
		Fields: map[string]*core.MappingNode{
			"testField": core.MappingNodeFromString("testValue"),
		},
	}

	target := &testTarget{}

	setter.Set(specData, target)

	s.Assert().Equal("testValue", target.FieldValue)
	s.Assert().True(setter.DidSet())
}

func (s *SpecValueSetterSuite) Test_skips_setting_value_for_non_existent_path() {
	setter := SpecValueSetter[*testTarget]{
		PathInSpec: "$.testFieldMissing",
		SetValueFunc: func(value *core.MappingNode, target *testTarget) {
			target.FieldValue = core.StringValue(value)
		},
	}

	specData := &core.MappingNode{
		Fields: map[string]*core.MappingNode{
			"testField": core.MappingNodeFromString("testValue"),
		},
	}

	target := &testTarget{}

	setter.Set(specData, target)

	s.Assert().False(setter.DidSet())
	s.Assert().Equal("", target.FieldValue)
}

func (s *SpecValueSetterSuite) Test_skips_setting_value_for_value_explicity_set_to_nil() {
	setter := SpecValueSetter[*testTarget]{
		PathInSpec: "$.testField",
		SetValueFunc: func(value *core.MappingNode, target *testTarget) {
			target.FieldValue = core.StringValue(value)
		},
	}

	specData := &core.MappingNode{
		Fields: map[string]*core.MappingNode{
			"testField": nil,
		},
	}

	target := &testTarget{}

	setter.Set(specData, target)

	s.Assert().False(setter.DidSet())
	s.Assert().Equal("", target.FieldValue)
}

func TestSpecValueSetterSuite(t *testing.T) {
	suite.Run(t, new(SpecValueSetterSuite))
}

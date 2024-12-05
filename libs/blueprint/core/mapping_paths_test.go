package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type MappingPathsTestSuite struct {
	suite.Suite
}

func (s *MappingPathsTestSuite) Test_get_value_by_path_for_complex_path() {
	path := "$[\"cluster\\\".v1\"].config.environments[0].hosts[0].endpoint"
	node, expectedEndpoint := fixtureMappingNode1()
	value, err := GetPathValue(path, node, 10)
	s.Require().NoError(err)
	s.Assert().Equal(&MappingNode{
		Scalar: &ScalarValue{
			StringValue: &expectedEndpoint,
		},
	}, value)
}

func (s *MappingPathsTestSuite) Test_returns_passed_in_node_for_root_identity_path() {
	path := "$"
	node, _ := fixtureMappingNode1()
	value, err := GetPathValue(path, node, 10)
	s.Require().NoError(err)
	s.Assert().Equal(node, value)
}

func (s *MappingPathsTestSuite) Test_returns_nil_for_non_existent_path() {
	path := "$[\"cluster\\\".v1\"].config.environments[0].hosts[0].missingField"
	node, _ := fixtureMappingNode1()
	value, err := GetPathValue(path, node, 10)
	s.Require().NoError(err)
	s.Assert().Nil(value)
}

func (s *MappingPathsTestSuite) Test_returns_nil_for_path_that_goes_beyond_max_depth() {
	node, path := fixtureDeepMappingNode(30)
	value, err := GetPathValue(path, node, 10)
	s.Require().NoError(err)
	s.Assert().Nil(value)
}

func (s *MappingPathsTestSuite) Test_fails_to_get_value_by_path_for_invalid_path() {
	path := "$[\"cluster\\\".v1\"].config.environments[0unexpected].hosts[0].missingField"
	node, _ := fixtureMappingNode1()
	_, err := GetPathValue(path, node, 10)
	s.Require().Error(err)
	mappingPathErr, isMappingPathErr := err.(*MappingPathError)
	s.Require().True(isMappingPathErr)
	s.Assert().Equal(ErrInvalidMappingPath, mappingPathErr.ReasonCode)
	s.Assert().Equal(path, mappingPathErr.Path)
}

func (s *MappingPathsTestSuite) Test_fails_to_get_value_by_path_for_invalid_path_2() {
	// Missing $ for the root object of the path.
	path := ".config[\"hosts\"][0]"
	node, _ := fixtureMappingNode1()
	_, err := GetPathValue(path, node, 10)
	s.Require().Error(err)
	mappingPathErr, isMappingPathErr := err.(*MappingPathError)
	s.Require().True(isMappingPathErr)
	s.Assert().Equal(ErrInvalidMappingPath, mappingPathErr.ReasonCode)
	s.Assert().Equal(path, mappingPathErr.Path)
}

func fixtureMappingNode1() (*MappingNode, string) {
	endpoint := "https://sfg94832-api.example.com"
	return &MappingNode{
		Fields: map[string]*MappingNode{
			"cluster\".v1": {
				Fields: map[string]*MappingNode{
					"config": {
						Fields: map[string]*MappingNode{
							"environments": {
								Items: []*MappingNode{
									{
										Fields: map[string]*MappingNode{
											"hosts": {
												Items: []*MappingNode{
													{
														Fields: map[string]*MappingNode{
															"endpoint": {
																Scalar: &ScalarValue{
																	StringValue: &endpoint,
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}, endpoint
}

func fixtureDeepMappingNode(depth int) (*MappingNode, string) {
	node := &MappingNode{}
	path := "$"
	current := node
	for i := 0; i < depth; i++ {
		fieldName := fmt.Sprintf("field%d", i)
		path += "." + fieldName
		current.Fields = map[string]*MappingNode{
			fieldName: {
				Fields: map[string]*MappingNode{},
			},
		}
		current = current.Fields[fieldName]
	}

	return node, path
}

func TestMappingPathsTestSuite(t *testing.T) {
	suite.Run(t, new(MappingPathsTestSuite))
}

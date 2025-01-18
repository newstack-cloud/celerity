package container

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
)

type DependencyUtilsTestSuite struct {
	suite.Suite
}

func (s *DependencyUtilsTestSuite) Test_populates_dependency_for_linked_to_resource() {
	saveOrderFunctionNode := &DeploymentNode{
		ChainLinkNode: &links.ChainLinkNode{
			ResourceName: "saveOrderFunction",
			LinksTo: []*links.ChainLinkNode{
				{
					ResourceName: "ordersTable",
				},
			},
			LinkedFrom: []*links.ChainLinkNode{},
			LinkImplementations: map[string]provider.Link{
				"ordersTable": &testLambdaDynamoDBTableLink{},
			},
		},
		DirectDependencies: []*DeploymentNode{},
	}
	ordersTableNode := &DeploymentNode{
		ChainLinkNode: &links.ChainLinkNode{
			ResourceName: "ordersTable",
			LinkedFrom: []*links.ChainLinkNode{
				saveOrderFunctionNode.ChainLinkNode,
			},
			LinkImplementations: map[string]provider.Link{},
		},
		DirectDependencies: []*DeploymentNode{},
	}
	nodes := []*DeploymentNode{
		saveOrderFunctionNode,
		ordersTableNode,
	}
	err := PopulateDirectDependencies(
		context.Background(),
		nodes,
		validation.NewRefChainCollector(),
		s.createBlueprintParams(),
	)
	s.Require().NoError(err)
	s.Assert().Equal(nodes[0].DirectDependencies, []*DeploymentNode{
		ordersTableNode,
	})
}

func (s *DependencyUtilsTestSuite) Test_reports_resource_does_not_depend_on_linked_to_resource() {
	saveOrderFunctionNode := &DeploymentNode{
		ChainLinkNode: &links.ChainLinkNode{
			ResourceName: "saveOrderFunction",
			LinksTo: []*links.ChainLinkNode{
				{
					ResourceName: "preprocessOrderFunction",
				},
			},
			LinkedFrom: []*links.ChainLinkNode{},
			LinkImplementations: map[string]provider.Link{
				// Lambda -> Lambda link does not have a priority
				// resource, therefore it should not be considered
				// as a dependency.
				"preprocessOrderFunction": &testLambdaLambdaLink{},
			},
		},
	}
	preprocessOrderFunctionNode := &DeploymentNode{
		ChainLinkNode: &links.ChainLinkNode{
			ResourceName: "preprocessOrderFunction",
			LinkedFrom: []*links.ChainLinkNode{
				saveOrderFunctionNode.ChainLinkNode,
			},
			LinkImplementations: map[string]provider.Link{},
		},
	}
	nodes := []*DeploymentNode{
		saveOrderFunctionNode,
		preprocessOrderFunctionNode,
	}
	err := PopulateDirectDependencies(
		context.Background(),
		nodes,
		validation.NewRefChainCollector(),
		s.createBlueprintParams(),
	)
	s.Require().NoError(err)
	s.Assert().Empty(nodes[0].DirectDependencies)
}

// func (s *DependencyUtilsTestSuite) Test_reports_resource_depends_on_linked_from_resource() {
// 	hasDependency, err := CheckHasDependencyOnResource(
// 		context.Background(),
// 		&DeploymentNode{
// 			ChainLinkNode: &links.ChainLinkNode{
// 				ResourceName: "saveOrderFunction",
// 				LinksTo:      []*links.ChainLinkNode{},
// 				LinkedFrom: []*links.ChainLinkNode{
// 					{
// 						ResourceName: "ordersTable",
// 						LinkImplementations: map[string]provider.Link{
// 							"saveOrderFunction": &testDynamoDBTableLambdaLink{},
// 						},
// 					},
// 				},
// 			},
// 		},
// 		"ordersTable",
// 		validation.NewRefChainCollector(),
// 		s.createBlueprintParams(),
// 	)
// 	s.Require().NoError(err)
// 	s.True(hasDependency)
// }

// func (s *DependencyUtilsTestSuite) Test_reports_resource_does_not_depend_on_linked_from_resource() {
// 	hasDependency, err := CheckHasDependencyOnResource(
// 		context.Background(),
// 		&DeploymentNode{
// 			ChainLinkNode: &links.ChainLinkNode{
// 				ResourceName: "saveOrderFunction",
// 				LinksTo:      []*links.ChainLinkNode{},
// 				LinkedFrom: []*links.ChainLinkNode{
// 					{
// 						ResourceName: "preprocessOrderFunction",
// 						LinkImplementations: map[string]provider.Link{
// 							"saveOrderFunction": &testLambdaLambdaLink{},
// 						},
// 					},
// 				},
// 			},
// 		},
// 		"preprocessOrderFunction",
// 		validation.NewRefChainCollector(),
// 		s.createBlueprintParams(),
// 	)
// 	s.Require().NoError(err)
// 	s.False(hasDependency)
// }

// func (s *DependencyUtilsTestSuite) Test_reports_resource_depends_on_referenced_resource() {
// 	refChainCollector := validation.NewRefChainCollector()
// 	refChainCollector.Collect(
// 		core.ResourceElementID("ordersTable"),
// 		/* element */ nil,
// 		core.ResourceElementID("saveOrderFunction"),
// 		/* tags */ []string{},
// 	)

// 	hasDependency, err := CheckHasDependencyOnResource(
// 		context.Background(),
// 		&DeploymentNode{
// 			ChainLinkNode: &links.ChainLinkNode{
// 				ResourceName: "saveOrderFunction",
// 				LinksTo:      []*links.ChainLinkNode{},
// 				LinkedFrom:   []*links.ChainLinkNode{},
// 			},
// 		},
// 		"ordersTable",
// 		refChainCollector,
// 		s.createBlueprintParams(),
// 	)
// 	s.Require().NoError(err)
// 	s.True(hasDependency)
// }

// func (s *DependencyUtilsTestSuite) Test_reports_resource_does_not_depend_on_referenced_resource() {
// 	// Empty reference chain collector should not have any
// 	// references to the resource.
// 	refChainCollector := validation.NewRefChainCollector()

// 	hasDependency, err := CheckHasDependencyOnResource(
// 		context.Background(),
// 		&DeploymentNode{
// 			ChainLinkNode: &links.ChainLinkNode{
// 				ResourceName: "saveOrderFunction",
// 				LinksTo:      []*links.ChainLinkNode{},
// 				LinkedFrom:   []*links.ChainLinkNode{},
// 			},
// 		},
// 		"ordersTable",
// 		refChainCollector,
// 		s.createBlueprintParams(),
// 	)
// 	s.Require().NoError(err)
// 	s.False(hasDependency)
// }

// func (s *DependencyUtilsTestSuite) Test_reports_resource_depends_on_referenced_child_blueprint() {
// 	refChainCollector := validation.NewRefChainCollector()
// 	refChainCollector.Collect(
// 		core.ChildElementID("coreInfra"),
// 		/* element */ nil,
// 		core.ResourceElementID("saveOrderFunction"),
// 		/* tags */ []string{},
// 	)

// 	hasDependency, err := CheckHasDependencyOnChildBlueprint(
// 		context.Background(),
// 		&DeploymentNode{
// 			ChainLinkNode: &links.ChainLinkNode{
// 				ResourceName: "saveOrderFunction",
// 				LinksTo:      []*links.ChainLinkNode{},
// 				LinkedFrom:   []*links.ChainLinkNode{},
// 			},
// 		},
// 		"coreInfra",
// 		refChainCollector,
// 		s.createBlueprintParams(),
// 	)
// 	s.Require().NoError(err)
// 	s.True(hasDependency)
// }

// func (s *DependencyUtilsTestSuite) Test_reports_resource_does_not_depend_on_referenced_child_blueprint() {
// 	// Empty reference chain collector should not have any
// 	// references to the child blueprint.
// 	refChainCollector := validation.NewRefChainCollector()

// 	hasDependency, err := CheckHasDependencyOnChildBlueprint(
// 		context.Background(),
// 		&DeploymentNode{
// 			ChainLinkNode: &links.ChainLinkNode{
// 				ResourceName: "saveOrderFunction",
// 				LinksTo:      []*links.ChainLinkNode{},
// 				LinkedFrom:   []*links.ChainLinkNode{},
// 			},
// 		},
// 		"coreInfra",
// 		refChainCollector,
// 		s.createBlueprintParams(),
// 	)
// 	s.Require().NoError(err)
// 	s.False(hasDependency)
// }

// func (s *DependencyUtilsTestSuite) Test_reports_child_depends_on_referenced_child_blueprint() {
// 	hasDependency, err := CheckHasDependencyOnChildBlueprint(
// 		context.Background(),
// 		&DeploymentNode{
// 			ChildNode: &validation.ReferenceChainNode{
// 				ElementName: core.ChildElementID("networking"),
// 				References: []*validation.ReferenceChainNode{
// 					{
// 						ElementName: core.ChildElementID("coreInfra"),
// 						References:  []*validation.ReferenceChainNode{},
// 					},
// 				},
// 			},
// 		},
// 		"coreInfra",
// 		validation.NewRefChainCollector(),
// 		s.createBlueprintParams(),
// 	)
// 	s.Require().NoError(err)
// 	s.True(hasDependency)
// }

// func (s *DependencyUtilsTestSuite) Test_reports_child_does_not_depends_on_referenced_child_blueprint() {
// 	hasDependency, err := CheckHasDependencyOnChildBlueprint(
// 		context.Background(),
// 		&DeploymentNode{
// 			ChildNode: &validation.ReferenceChainNode{
// 				ElementName: core.ChildElementID("networking"),
// 				References: []*validation.ReferenceChainNode{
// 					{
// 						ElementName: core.ChildElementID("coreInfrav1"),
// 						References:  []*validation.ReferenceChainNode{},
// 					},
// 				},
// 			},
// 		},
// 		"coreInfrav2",
// 		validation.NewRefChainCollector(),
// 		s.createBlueprintParams(),
// 	)
// 	s.Require().NoError(err)
// 	s.False(hasDependency)
// }

// func (s *DependencyUtilsTestSuite) Test_reports_child_depends_on_referenced_resource() {
// 	hasDependency, err := CheckHasDependencyOnResource(
// 		context.Background(),
// 		&DeploymentNode{
// 			ChildNode: &validation.ReferenceChainNode{
// 				ElementName: core.ChildElementID("networking"),
// 				References: []*validation.ReferenceChainNode{
// 					{
// 						ElementName: core.ResourceElementID("saveOrderFunction"),
// 						References:  []*validation.ReferenceChainNode{},
// 					},
// 				},
// 			},
// 		},
// 		"saveOrderFunction",
// 		validation.NewRefChainCollector(),
// 		s.createBlueprintParams(),
// 	)
// 	s.Require().NoError(err)
// 	s.True(hasDependency)
// }

// func (s *DependencyUtilsTestSuite) Test_reports_child_does_not_depend_on_referenced_resource() {
// 	hasDependency, err := CheckHasDependencyOnResource(
// 		context.Background(),
// 		&DeploymentNode{
// 			ChildNode: &validation.ReferenceChainNode{
// 				ElementName: core.ChildElementID("networking"),
// 				References: []*validation.ReferenceChainNode{
// 					{
// 						ElementName: core.ResourceElementID("preprocessOrderFunction"),
// 						References:  []*validation.ReferenceChainNode{},
// 					},
// 				},
// 			},
// 		},
// 		"saveOrderFunction",
// 		validation.NewRefChainCollector(),
// 		s.createBlueprintParams(),
// 	)
// 	s.Require().NoError(err)
// 	s.False(hasDependency)
// }

func (s *DependencyUtilsTestSuite) createBlueprintParams() core.BlueprintParams {
	return core.NewDefaultParams(
		map[string]map[string]*core.ScalarValue{},
		map[string]map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
	)
}

func TestDependencyUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(DependencyUtilsTestSuite))
}

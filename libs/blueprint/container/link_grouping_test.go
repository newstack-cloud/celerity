package container

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
)

type GroupOrderedLinkNodesTestSuite struct {
	groupFixture1 groupChainLinkNodeFixture
	groupFixture2 groupChainLinkNodeFixture
	groupFixture3 groupChainLinkNodeFixture
	suite.Suite
}

type groupChainLinkNodeFixture struct {
	orderedLinkNodes  []*links.ChainLinkNode
	refChainCollector validation.RefChainCollector
	// All the resource names that are expected to be in each group.
	// The order of the groups matter but the order of the resources
	// in each group doesn't.
	expectedPresent [][]string
}

func (s *GroupOrderedLinkNodesTestSuite) SetupSuite() {
	groupFixture1, err := groupFixture1()
	if err != nil {
		s.FailNow(err.Error())
	}
	s.groupFixture1 = groupFixture1

	groupFixture2, err := groupFixture2()
	if err != nil {
		s.FailNow(err.Error())
	}
	s.groupFixture2 = groupFixture2

	groupFixture3, err := groupFixture3()
	if err != nil {
		s.FailNow(err.Error())
	}
	s.groupFixture3 = groupFixture3
}

func (s *GroupOrderedLinkNodesTestSuite) Test_group_links_for_deployment_with_circular_links() {
	groups, err := GroupOrderedLinkNodes(
		context.TODO(),
		s.groupFixture1.orderedLinkNodes,
		s.groupFixture1.refChainCollector,
		nil,
	)
	s.Assert().NoError(err)
	s.Assert().Len(groups, len(s.groupFixture1.expectedPresent))

	s.assertExpectedGroups(groups, s.groupFixture1.expectedPresent)
}

func (s *GroupOrderedLinkNodesTestSuite) Test_group_links_for_deployment_without_circular_links() {
	groups, err := GroupOrderedLinkNodes(
		context.TODO(),
		s.groupFixture2.orderedLinkNodes,
		s.groupFixture2.refChainCollector,
		nil,
	)
	s.Assert().NoError(err)
	s.Assert().Len(groups, len(s.groupFixture2.expectedPresent))

	s.assertExpectedGroups(groups, s.groupFixture2.expectedPresent)
}

func (s *GroupOrderedLinkNodesTestSuite) Test_group_links_for_deployment_based_on_references() {
	groups, err := GroupOrderedLinkNodes(
		context.TODO(),
		s.groupFixture3.orderedLinkNodes,
		s.groupFixture3.refChainCollector,
		nil,
	)
	s.Assert().NoError(err)
	s.Assert().Len(groups, len(s.groupFixture3.expectedPresent))

	s.assertExpectedGroups(groups, s.groupFixture3.expectedPresent)
}

func (s *GroupOrderedLinkNodesTestSuite) assertExpectedGroups(
	groups [][]*links.ChainLinkNode,
	expectedPresent [][]string,
) {
	for i, group := range groups {
		expectedGroupNames := expectedPresent[i]
		expectedGroupNamesNormalised := []string{}
		copy(expectedGroupNamesNormalised, expectedGroupNames)
		groupNormalised := []string{}
		for _, node := range group {
			groupNormalised = append(groupNormalised, node.ResourceName)
		}
		slices.Sort(groupNormalised)
		slices.Sort(expectedGroupNames)
		s.Assert().Equal(expectedGroupNames, groupNormalised)

	}
}

var testGroupProviderImpl = newTestAWSProvider()

func groupFixture1() (groupChainLinkNodeFixture, error) {
	var orderedLinkNodes = groupFixture1Chains()
	refChainCollector, err := groupFixture1RefChains(orderedLinkNodes)
	if err != nil {
		return groupChainLinkNodeFixture{}, err
	}

	return groupChainLinkNodeFixture{
		orderedLinkNodes:  orderedLinkNodes,
		refChainCollector: refChainCollector,
		expectedPresent: [][]string{
			{
				"orderApi",
				"ordersTable",
			},
			{
				"ordersStream",
				"getOrdersFunction",
				"createOrderFunction",
				"updateOrderFunction",
				// The link between statsAccumulatorFunction and ordersStream
				// is a soft link in the test provider so they can be resolved concurrently.
				"statsAccumulatorFunction",
			},
		},
	}, nil
}

func groupFixture1Chains() []*links.ChainLinkNode {
	apiGatewayLambdaLinkImpl, _ := testGroupProviderImpl.Link(context.TODO(), "aws/apigateway/api", "aws/lambda/function")
	orderApi := &links.ChainLinkNode{
		ResourceName: "orderApi",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/apigateway/api"},
		},
		Paths: []string{},
		LinkImplementations: map[string]provider.Link{
			"getOrdersFunction":   apiGatewayLambdaLinkImpl,
			"createOrderFunction": apiGatewayLambdaLinkImpl,
			"updateOrderFunction": apiGatewayLambdaLinkImpl,
		},
		LinkedFrom: []*links.ChainLinkNode{},
		LinksTo:    []*links.ChainLinkNode{},
	}

	lambdaDynamoDBTableLink, _ := testProviderImpl.Link(context.TODO(), "aws/lambda/function", "aws/dynamodb/table")
	getOrdersFunction := &links.ChainLinkNode{
		ResourceName: "getOrdersFunction",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/lambda/function"},
		},
		Paths: []string{"/orderApi"},
		LinkImplementations: map[string]provider.Link{
			"ordersTable": lambdaDynamoDBTableLink,
		},
		LinkedFrom: []*links.ChainLinkNode{
			orderApi,
		},
	}
	createOrderFunction := &links.ChainLinkNode{
		ResourceName: "createOrderFunction",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/lambda/function"},
		},
		LinkImplementations: map[string]provider.Link{
			"ordersTable": lambdaDynamoDBTableLink,
		},
		Paths: []string{"/orderApi"},
		LinkedFrom: []*links.ChainLinkNode{
			orderApi,
		},
	}
	updateOrderFunction := &links.ChainLinkNode{
		ResourceName: "updateOrderFunction",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/lambda/function"},
		},
		LinkImplementations: map[string]provider.Link{
			"ordersTable": lambdaDynamoDBTableLink,
		},
		Paths: []string{"/orderApi"},
		LinkedFrom: []*links.ChainLinkNode{
			orderApi,
		},
	}

	dynamoDBTableStreamLink, _ := testProviderImpl.Link(context.TODO(), "aws/dynamodb/table", "aws/dynamodb/stream")
	// The only hard link in this chain is between the orders table
	// and the orders stream.
	ordersTable := &links.ChainLinkNode{
		ResourceName: "ordersTable",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/dynamodb/table"},
		},
		LinkImplementations: map[string]provider.Link{
			"ordersStream": dynamoDBTableStreamLink,
		},
		Paths: []string{
			"/orderApi/getOrdersFunction",
			"/orderApi/createOrderFunction",
			"/orderApi/updateOrderFunction",
		},
		LinkedFrom: []*links.ChainLinkNode{
			getOrdersFunction,
		},
	}

	dynamoDBStreamLambdaLink, _ := testProviderImpl.Link(context.TODO(), "aws/dynamodb/stream", "aws/lambda/function")
	ordersStream := &links.ChainLinkNode{
		ResourceName: "ordersStream",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/dynamodb/stream"},
		},
		Paths: []string{
			"/orderApi/getOrdersFunction/ordersTable",
			"/orderApi/createOrderFunction/ordersTable",
			"/orderApi/updateOrderFunction/ordersTable",
		},
		LinkImplementations: map[string]provider.Link{
			"statsAccumulatorFunction": dynamoDBStreamLambdaLink,
		},
		LinkedFrom: []*links.ChainLinkNode{
			getOrdersFunction,
			createOrderFunction,
			updateOrderFunction,
			ordersTable,
		},
		LinksTo: []*links.ChainLinkNode{},
	}

	// Includes transitive soft circular link.
	statsAccumulatorFunction := &links.ChainLinkNode{
		ResourceName: "statsAccumulatorFunction",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/lambda/function"},
		},
		Paths: []string{
			"/orderApi/getOrdersFunction/ordersTable/ordersStream",
			"/orderApi/createOrderFunction/ordersTable/ordersStream",
			"/orderApi/updateOrderFunction/ordersTable/ordersStream",
		},
		LinkImplementations: map[string]provider.Link{
			"ordersTable": lambdaDynamoDBTableLink,
		},
		LinkedFrom: []*links.ChainLinkNode{
			ordersStream,
		},
	}

	orderApi.LinksTo = []*links.ChainLinkNode{
		getOrdersFunction,
		createOrderFunction,
		updateOrderFunction,
	}
	getOrdersFunction.LinksTo = []*links.ChainLinkNode{
		ordersTable,
	}
	createOrderFunction.LinksTo = []*links.ChainLinkNode{
		ordersTable,
	}
	updateOrderFunction.LinksTo = []*links.ChainLinkNode{
		ordersTable,
	}
	ordersTable.LinksTo = []*links.ChainLinkNode{
		ordersStream,
	}
	ordersStream.LinksTo = []*links.ChainLinkNode{
		statsAccumulatorFunction,
	}
	statsAccumulatorFunction.LinksTo = []*links.ChainLinkNode{
		ordersTable,
	}

	return []*links.ChainLinkNode{
		orderApi,
		ordersTable,
		ordersStream,
		getOrdersFunction,
		createOrderFunction,
		updateOrderFunction,
		statsAccumulatorFunction,
	}
}

func groupFixture1RefChains(
	linkChains []*links.ChainLinkNode,
) (validation.RefChainCollector, error) {
	collector := validation.NewRefChainCollector()
	for _, link := range linkChains {
		err := collectLinksFromChain(context.TODO(), link, collector)
		if err != nil {
			return nil, err
		}
	}

	collector.Collect(
		"resources.ordersTable",
		nil,
		"resources.getOrdersFunction",
		[]string{"subRef:resources.getOrdersFunction"},
	)

	return collector, nil
}

func groupFixture2() (groupChainLinkNodeFixture, error) {
	var orderedLinkNodes = groupFixture2Chain()
	refChainCollector, err := groupFixture2RefChains(orderedLinkNodes)
	if err != nil {
		return groupChainLinkNodeFixture{}, err
	}

	return groupChainLinkNodeFixture{
		orderedLinkNodes:  orderedLinkNodes,
		refChainCollector: refChainCollector,
		expectedPresent: [][]string{
			{
				"vpc1",
			},
			{
				"routeTable1",
				"igw1",
			},
			{
				"route1",
				"subnet1",
				"sg1",
			},
		},
	}, nil
}

func groupFixture2Chain() []*links.ChainLinkNode {
	routeRouteTableLink, _ := testGroupProviderImpl.Link(context.TODO(), "aws/ec2/route", "aws/ec2/routeTable")
	routeIGWLink, _ := testProviderImpl.Link(context.TODO(), "aws/ec2/route", "aws/ec2/internetGateway")
	route := &links.ChainLinkNode{
		ResourceName: "route1",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/ec2/route"},
		},
		Paths: []string{},
		LinkImplementations: map[string]provider.Link{
			"routeTable1": routeRouteTableLink,
			"igw1":        routeIGWLink,
		},
		LinkedFrom: []*links.ChainLinkNode{},
		LinksTo:    []*links.ChainLinkNode{},
	}

	routeTableVPCLink, _ := testProviderImpl.Link(context.TODO(), "aws/ec2/routeTable", "aws/ec2/vpc")
	routeTable := &links.ChainLinkNode{
		ResourceName: "routeTable1",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/ec2/routeTable"},
		},
		Paths: []string{
			"/route1",
		},
		LinkImplementations: map[string]provider.Link{
			"vpc1": routeTableVPCLink,
		},
		LinkedFrom: []*links.ChainLinkNode{
			route,
		},
		LinksTo: []*links.ChainLinkNode{},
	}

	internetGateway := &links.ChainLinkNode{
		ResourceName: "igw1",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/ec2/internetGateway"},
		},
		Paths: []string{
			"/route1",
		},
		LinkImplementations: map[string]provider.Link{},
		LinkedFrom: []*links.ChainLinkNode{
			route,
		},
		LinksTo: []*links.ChainLinkNode{},
	}

	subnetVPCLink, _ := testProviderImpl.Link(context.TODO(), "aws/ec2/subnet", "aws/ec2/vpc")
	subnet := &links.ChainLinkNode{
		ResourceName: "subnet1",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/ec2/subnet"},
		},
		Paths: []string{},
		LinkImplementations: map[string]provider.Link{
			"vpc1": subnetVPCLink,
		},
		LinkedFrom: []*links.ChainLinkNode{},
		LinksTo:    []*links.ChainLinkNode{},
	}

	securityGroupLink, _ := testProviderImpl.Link(context.TODO(), "aws/ec2/securityGroup", "aws/ec2/vpc")
	securityGroup := &links.ChainLinkNode{
		ResourceName: "sg1",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/ec2/securityGroup"},
		},
		Paths: []string{},
		LinkImplementations: map[string]provider.Link{
			"vpc1": securityGroupLink,
		},
		LinkedFrom: []*links.ChainLinkNode{},
		LinksTo:    []*links.ChainLinkNode{},
	}

	vpc := &links.ChainLinkNode{
		ResourceName: "vpc1",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/ec2/vpc"},
		},
		Paths: []string{
			"/route1/routeTable1",
			"/subnet1",
			"/sg1",
		},
		LinkImplementations: map[string]provider.Link{},
		LinkedFrom: []*links.ChainLinkNode{
			routeTable,
			subnet,
			securityGroup,
		},
		LinksTo: []*links.ChainLinkNode{},
	}

	route.LinksTo = []*links.ChainLinkNode{
		routeTable,
		internetGateway,
	}
	routeTable.LinksTo = []*links.ChainLinkNode{
		vpc,
	}
	subnet.LinksTo = []*links.ChainLinkNode{
		vpc,
	}
	securityGroup.LinksTo = []*links.ChainLinkNode{
		vpc,
	}

	return []*links.ChainLinkNode{
		vpc,
		routeTable,
		internetGateway,
		route,
		subnet,
		securityGroup,
	}
}

func groupFixture2RefChains(
	linkChains []*links.ChainLinkNode,
) (validation.RefChainCollector, error) {
	collector := validation.NewRefChainCollector()
	for _, link := range linkChains {
		err := collectLinksFromChain(context.TODO(), link, collector)
		if err != nil {
			return nil, err
		}
	}

	collector.Collect("resources.vpc1", nil, "resources.sg1", []string{"subRef:resources.sg1"})

	return collector, nil
}

func groupFixture3() (groupChainLinkNodeFixture, error) {
	var orderedLinkNodes = groupFixture3Chains()
	refChainCollector, err := groupFixture3RefChains(orderedLinkNodes)
	if err != nil {
		return groupChainLinkNodeFixture{}, err
	}

	return groupChainLinkNodeFixture{
		orderedLinkNodes:  orderedLinkNodes,
		refChainCollector: refChainCollector,
		expectedPresent: [][]string{
			{
				"ordersTable",
				"orderApi",
			},
			{
				"ordersStream",
				// The link between statsAccumulatorFunction and ordersStream
				// is a soft link in the test provider so they can be resolved concurrently.
				"statsAccumulatorFunction",
				"getOrdersFunction",
				"createOrderFunction",
				"updateOrderFunction",
			},
		},
	}, nil
}

func groupFixture3Chains() []*links.ChainLinkNode {
	apiGatewayLambdaLinkImpl, _ := testGroupProviderImpl.Link(context.TODO(), "aws/apigateway/api", "aws/lambda/function")
	orderApi := &links.ChainLinkNode{
		ResourceName: "orderApi",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/apigateway/api"},
		},
		Paths: []string{},
		LinkImplementations: map[string]provider.Link{
			"getOrdersFunction":   apiGatewayLambdaLinkImpl,
			"createOrderFunction": apiGatewayLambdaLinkImpl,
			"updateOrderFunction": apiGatewayLambdaLinkImpl,
		},
		LinkedFrom: []*links.ChainLinkNode{},
		LinksTo:    []*links.ChainLinkNode{},
	}

	lambdaDynamoDBTableLink, _ := testProviderImpl.Link(context.TODO(), "aws/lambda/function", "aws/dynamodb/table")
	// For fixture 3, functions do not link to tables, references are being tested for this fixture
	// so each function will have a reference to the orders table defined in the ref chain fixtures.
	getOrdersFunction := &links.ChainLinkNode{
		ResourceName: "getOrdersFunction",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/lambda/function"},
		},
		Paths:               []string{"/orderApi"},
		LinkImplementations: map[string]provider.Link{},
		LinkedFrom: []*links.ChainLinkNode{
			orderApi,
		},
	}
	createOrderFunction := &links.ChainLinkNode{
		ResourceName: "createOrderFunction",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/lambda/function"},
		},
		LinkImplementations: map[string]provider.Link{},
		Paths:               []string{"/orderApi"},
		LinkedFrom: []*links.ChainLinkNode{
			orderApi,
		},
	}
	updateOrderFunction := &links.ChainLinkNode{
		ResourceName: "updateOrderFunction",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/lambda/function"},
		},
		LinkImplementations: map[string]provider.Link{},
		Paths:               []string{"/orderApi"},
		LinkedFrom: []*links.ChainLinkNode{
			orderApi,
		},
	}

	dynamoDBTableStreamLink, _ := testProviderImpl.Link(context.TODO(), "aws/dynamodb/table", "aws/dynamodb/stream")
	// The only hard link in this chain is between the orders table
	// and the orders stream.
	ordersTable := &links.ChainLinkNode{
		ResourceName: "ordersTable",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/dynamodb/table"},
		},
		LinkImplementations: map[string]provider.Link{
			"ordersStream": dynamoDBTableStreamLink,
		},
		Paths:      []string{},
		LinkedFrom: []*links.ChainLinkNode{},
	}

	dynamoDBStreamLambdaLink, _ := testProviderImpl.Link(context.TODO(), "aws/dynamodb/stream", "aws/lambda/function")
	ordersStream := &links.ChainLinkNode{
		ResourceName: "ordersStream",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/dynamodb/stream"},
		},
		Paths: []string{
			"/ordersTable",
		},
		LinkImplementations: map[string]provider.Link{
			"statsAccumulatorFunction": dynamoDBStreamLambdaLink,
		},
		LinkedFrom: []*links.ChainLinkNode{
			ordersTable,
		},
		LinksTo: []*links.ChainLinkNode{},
	}

	// Includes transitive soft circular link.
	statsAccumulatorFunction := &links.ChainLinkNode{
		ResourceName: "statsAccumulatorFunction",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/lambda/function"},
		},
		Paths: []string{
			"/ordersTable/ordersStream",
		},
		LinkImplementations: map[string]provider.Link{
			"ordersTable": lambdaDynamoDBTableLink,
		},
		LinkedFrom: []*links.ChainLinkNode{
			ordersStream,
		},
	}

	orderApi.LinksTo = []*links.ChainLinkNode{
		getOrdersFunction,
		createOrderFunction,
		updateOrderFunction,
	}
	ordersTable.LinksTo = []*links.ChainLinkNode{
		ordersStream,
	}
	ordersStream.LinksTo = []*links.ChainLinkNode{
		statsAccumulatorFunction,
	}
	statsAccumulatorFunction.LinksTo = []*links.ChainLinkNode{
		ordersTable,
	}

	return []*links.ChainLinkNode{
		orderApi,
		ordersTable,
		getOrdersFunction,
		createOrderFunction,
		updateOrderFunction,
		ordersStream,
		statsAccumulatorFunction,
	}
}

func groupFixture3RefChains(
	linkChains []*links.ChainLinkNode,
) (validation.RefChainCollector, error) {
	collector := validation.NewRefChainCollector()
	for _, link := range linkChains {
		err := collectLinksFromChain(context.TODO(), link, collector)
		if err != nil {
			return nil, err
		}
	}

	collector.Collect(
		"resources.ordersTable",
		nil,
		"resources.getOrdersFunction",
		[]string{"subRef:resources.getOrdersFunction"},
	)
	collector.Collect(
		"resources.ordersTable",
		nil,
		"resources.createOrderFunction",
		[]string{"subRef:resources.createOrderFunction"},
	)
	collector.Collect(
		"resources.ordersTable",
		nil,
		"resources.updateOrderFunction",
		[]string{"subRef:resources.updateOrderFunction"},
	)

	return collector, nil
}

func TestGroupOrderedLinkNodesTestSuite(t *testing.T) {
	suite.Run(t, new(GroupOrderedLinkNodesTestSuite))
}

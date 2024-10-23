package container

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
	"github.com/two-hundred/celerity/libs/common/core"
)

type OrderingTestSuite struct {
	orderFixture1 orderChainLinkFixture
	orderFixture2 orderChainLinkFixture
	orderFixture3 orderChainLinkFixture
	suite.Suite
}

type orderChainLinkFixture struct {
	inputChains       []*links.ChainLink
	refChainCollector validation.RefChainCollector
	// All the resource names that are expected to be present
	// in the ordered flattened list of links (resources)
	// to be deployed.
	expectedPresent []string
	// A two-dimensional slice of resources with hard links that must
	// come in the provided order, it doesn't matter what the exact
	// positions in the list they are as long as they are in the given order.
	orderedExpected [][]string
}

func (s *OrderingTestSuite) SetupSuite() {
	orderFixture1, err := orderFixture1()
	if err != nil {
		s.FailNow(err.Error())
	}
	s.orderFixture1 = orderFixture1

	orderFixture2, err := orderFixture2()
	if err != nil {
		s.FailNow(err.Error())
	}
	s.orderFixture2 = orderFixture2

	orderFixture3, err := orderFixture3()
	if err != nil {
		s.FailNow(err.Error())
	}
	s.orderFixture3 = orderFixture3
}

func (s *OrderingTestSuite) Test_order_links_for_deployment_with_circular_links() {
	orderedLinks, err := OrderLinksForDeployment(
		context.TODO(),
		s.orderFixture1.inputChains,
		s.orderFixture1.refChainCollector,
		nil,
	)
	s.Assert().NoError(err)
	s.Assert().Len(orderedLinks, len(s.orderFixture1.expectedPresent))
	s.Assert().Len(
		core.Filter(
			orderedLinks,
			inExpected(s.orderFixture1.expectedPresent),
		),
		len(s.orderFixture1.expectedPresent),
	)

	for _, orderedExpectedSet := range s.orderFixture1.orderedExpected {
		s.assertOrderedExpected(orderedLinks, orderedExpectedSet)
	}
}

func (s *OrderingTestSuite) Test_order_links_for_deployment_without_circular_links() {
	orderedLinks, err := OrderLinksForDeployment(
		context.TODO(),
		s.orderFixture2.inputChains,
		s.orderFixture2.refChainCollector,
		nil,
	)
	s.Assert().NoError(err)
	s.Assert().Len(orderedLinks, len(s.orderFixture2.expectedPresent))
	s.Assert().Len(
		core.Filter(
			orderedLinks,
			inExpected(s.orderFixture2.expectedPresent),
		),
		len(s.orderFixture2.expectedPresent),
	)

	for _, orderedExpectedSet := range s.orderFixture2.orderedExpected {
		s.assertOrderedExpected(orderedLinks, orderedExpectedSet)
	}
}

func (s *OrderingTestSuite) Test_order_links_based_on_references() {
	orderedLinks, err := OrderLinksForDeployment(
		context.TODO(),
		s.orderFixture3.inputChains,
		s.orderFixture3.refChainCollector,
		nil,
	)
	s.Assert().NoError(err)
	s.Assert().Len(orderedLinks, len(s.orderFixture3.expectedPresent))
	s.Assert().Len(
		core.Filter(
			orderedLinks,
			inExpected(s.orderFixture3.expectedPresent),
		),
		len(s.orderFixture3.expectedPresent),
	)

	for _, orderedExpectedSet := range s.orderFixture3.orderedExpected {
		s.assertOrderedExpected(orderedLinks, orderedExpectedSet)
	}
}

func (s *OrderingTestSuite) assertOrderedExpected(actual []*links.ChainLink, orderedExpected []string) {
	expectedItemsInOrder := core.Filter(actual, inExpected(orderedExpected))
	inOrder := true
	i := 0
	var linkA *links.ChainLink
	var linkB *links.ChainLink

	for inOrder && i < len(expectedItemsInOrder) {
		if i+1 < len(expectedItemsInOrder) {
			linkA = expectedItemsInOrder[i]
			linkB = expectedItemsInOrder[i+1]
			inOrder = linkA.ResourceName == orderedExpected[i] && linkB.ResourceName == orderedExpected[i+1]
		}
		i += 2
	}

	if !inOrder {
		s.Failf("incorrect order", "expected \"%s\" to come before \"%s\"", linkB.ResourceName, linkA.ResourceName)
	}
}

func inExpected(expectedResourceNames []string) func(*links.ChainLink, int) bool {
	return func(currentLink *links.ChainLink, index int) bool {
		return core.SliceContainsComparable(expectedResourceNames, currentLink.ResourceName)
	}
}

var testProviderImpl = newTestAWSProvider()

func orderFixture1() (orderChainLinkFixture, error) {
	var inputChains = orderFixture1Chains()
	refChainCollector, err := orderFixture1RefChains(inputChains)
	if err != nil {
		return orderChainLinkFixture{}, err
	}

	return orderChainLinkFixture{
		inputChains:       inputChains,
		refChainCollector: refChainCollector,
		expectedPresent: []string{
			"orderApi",
			"getOrdersFunction",
			"createOrderFunction",
			"updateOrderFunction",
			"ordersTable",
			"ordersStream",
			"statsAccumulatorFunction",
		},
		orderedExpected: [][]string{{"ordersTable", "ordersStream"}},
	}, nil
}

func orderFixture1Chains() []*links.ChainLink {
	apiGatewayLambdaLinkImpl, _ := testProviderImpl.Link(context.TODO(), "aws/apigateway/api", "aws/lambda/function")
	orderApi := &links.ChainLink{
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
		LinkedFrom: []*links.ChainLink{},
		LinksTo:    []*links.ChainLink{},
	}

	lambdaDynamoDBTableLink, _ := testProviderImpl.Link(context.TODO(), "aws/lambda/function", "aws/dynamodb/table")
	getOrdersFunction := &links.ChainLink{
		ResourceName: "getOrdersFunction",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/lambda/function"},
		},
		Paths: []string{"/orderApi"},
		LinkImplementations: map[string]provider.Link{
			"ordersTable": lambdaDynamoDBTableLink,
		},
		LinkedFrom: []*links.ChainLink{
			orderApi,
		},
	}
	createOrderFunction := &links.ChainLink{
		ResourceName: "createOrderFunction",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/lambda/function"},
		},
		LinkImplementations: map[string]provider.Link{
			"ordersTable": lambdaDynamoDBTableLink,
		},
		Paths: []string{"/orderApi"},
		LinkedFrom: []*links.ChainLink{
			orderApi,
		},
	}
	updateOrderFunction := &links.ChainLink{
		ResourceName: "updateOrderFunction",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/lambda/function"},
		},
		LinkImplementations: map[string]provider.Link{
			"ordersTable": lambdaDynamoDBTableLink,
		},
		Paths: []string{"/orderApi"},
		LinkedFrom: []*links.ChainLink{
			orderApi,
		},
	}

	dynamoDBTableStreamLink, _ := testProviderImpl.Link(context.TODO(), "aws/dynamodb/table", "aws/dynamodb/stream")
	// The only hard link in this chain is between the orders table
	// and the orders stream.
	ordersTable := &links.ChainLink{
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
		LinkedFrom: []*links.ChainLink{
			getOrdersFunction,
		},
	}

	dynamoDBStreamLambdaLink, _ := testProviderImpl.Link(context.TODO(), "aws/dynamodb/stream", "aws/lambda/function")
	ordersStream := &links.ChainLink{
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
		LinkedFrom: []*links.ChainLink{
			getOrdersFunction,
			createOrderFunction,
			updateOrderFunction,
		},
		LinksTo: []*links.ChainLink{},
	}

	// Includes transitive soft circular link.
	statsAccumulatorFunction := &links.ChainLink{
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
		LinkedFrom: []*links.ChainLink{
			ordersStream,
		},
	}

	orderApi.LinksTo = []*links.ChainLink{
		getOrdersFunction,
		createOrderFunction,
		updateOrderFunction,
	}
	getOrdersFunction.LinksTo = []*links.ChainLink{
		ordersTable,
	}
	createOrderFunction.LinksTo = []*links.ChainLink{
		ordersTable,
	}
	updateOrderFunction.LinksTo = []*links.ChainLink{
		ordersTable,
	}
	ordersTable.LinksTo = []*links.ChainLink{
		ordersStream,
	}
	ordersStream.LinksTo = []*links.ChainLink{
		statsAccumulatorFunction,
	}
	statsAccumulatorFunction.LinksTo = []*links.ChainLink{
		ordersTable,
	}

	return []*links.ChainLink{
		orderApi,
	}
}

func orderFixture1RefChains(
	linkChains []*links.ChainLink,
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

func orderFixture2() (orderChainLinkFixture, error) {
	var inputChains = orderFixture2Chain()
	refChainCollector, err := orderFixture2RefChains(inputChains)
	if err != nil {
		return orderChainLinkFixture{}, err
	}

	return orderChainLinkFixture{
		inputChains:       inputChains,
		refChainCollector: refChainCollector,
		expectedPresent: []string{
			"route1",
			"subnet1",
			"sg1",
			"routeTable1",
			"vpc1",
			"igw1",
		},
		orderedExpected: [][]string{
			{"routeTable1", "route1"},
			{"igw1", "route1"},
			{"vpc1", "routeTable1"},
			{"vpc1", "subnet1"},
			{"vpc1", "sg1"},
		},
	}, nil
}

func orderFixture2Chain() []*links.ChainLink {
	routeRouteTableLink, _ := testProviderImpl.Link(context.TODO(), "aws/ec2/route", "aws/ec2/routeTable")
	routeIGWLink, _ := testProviderImpl.Link(context.TODO(), "aws/ec2/route", "aws/ec2/internetGateway")
	route := &links.ChainLink{
		ResourceName: "route1",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/ec2/route"},
		},
		Paths: []string{},
		LinkImplementations: map[string]provider.Link{
			"routeTable1": routeRouteTableLink,
			"igw1":        routeIGWLink,
		},
		LinkedFrom: []*links.ChainLink{},
		LinksTo:    []*links.ChainLink{},
	}

	routeTableVPCLink, _ := testProviderImpl.Link(context.TODO(), "aws/ec2/routeTable", "aws/ec2/vpc")
	routeTable := &links.ChainLink{
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
		LinkedFrom: []*links.ChainLink{
			route,
		},
		LinksTo: []*links.ChainLink{},
	}

	internetGateway := &links.ChainLink{
		ResourceName: "igw1",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/ec2/internetGateway"},
		},
		Paths: []string{
			"/route1",
		},
		LinkImplementations: map[string]provider.Link{},
		LinkedFrom: []*links.ChainLink{
			route,
		},
		LinksTo: []*links.ChainLink{},
	}

	subnetVPCLink, _ := testProviderImpl.Link(context.TODO(), "aws/ec2/subnet", "aws/ec2/vpc")
	subnet := &links.ChainLink{
		ResourceName: "subnet1",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/ec2/subnet"},
		},
		Paths: []string{},
		LinkImplementations: map[string]provider.Link{
			"vpc1": subnetVPCLink,
		},
		LinkedFrom: []*links.ChainLink{},
		LinksTo:    []*links.ChainLink{},
	}

	securityGroupLink, _ := testProviderImpl.Link(context.TODO(), "aws/ec2/securityGroup", "aws/ec2/vpc")
	securityGroup := &links.ChainLink{
		ResourceName: "sg1",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/ec2/securityGroup"},
		},
		Paths: []string{},
		LinkImplementations: map[string]provider.Link{
			"vpc1": securityGroupLink,
		},
		LinkedFrom: []*links.ChainLink{},
		LinksTo:    []*links.ChainLink{},
	}

	vpc := &links.ChainLink{
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
		LinkedFrom: []*links.ChainLink{
			routeTable,
			subnet,
			securityGroup,
		},
		LinksTo: []*links.ChainLink{},
	}

	route.LinksTo = []*links.ChainLink{
		routeTable,
		internetGateway,
	}
	routeTable.LinksTo = []*links.ChainLink{
		vpc,
	}
	subnet.LinksTo = []*links.ChainLink{
		vpc,
	}
	securityGroup.LinksTo = []*links.ChainLink{
		vpc,
	}

	return []*links.ChainLink{
		route,
		subnet,
		securityGroup,
	}
}

func orderFixture2RefChains(
	linkChains []*links.ChainLink,
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

func orderFixture3() (orderChainLinkFixture, error) {
	var inputChains = orderFixture3Chains()
	refChainCollector, err := orderFixture3RefChains(inputChains)
	if err != nil {
		return orderChainLinkFixture{}, err
	}

	return orderChainLinkFixture{
		inputChains:       inputChains,
		refChainCollector: refChainCollector,
		expectedPresent: []string{
			"orderApi",
			"getOrdersFunction",
			"createOrderFunction",
			"updateOrderFunction",
			"ordersTable",
			"ordersStream",
			"statsAccumulatorFunction",
		},
		orderedExpected: [][]string{
			{"ordersTable", "ordersStream"},
			{"ordersTable", "getOrdersFunction"},
			{"ordersTable", "createOrderFunction"},
			{"ordersTable", "updateOrderFunction"},
		},
	}, nil
}

func orderFixture3Chains() []*links.ChainLink {
	apiGatewayLambdaLinkImpl, _ := testProviderImpl.Link(context.TODO(), "aws/apigateway/api", "aws/lambda/function")
	orderApi := &links.ChainLink{
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
		LinkedFrom: []*links.ChainLink{},
		LinksTo:    []*links.ChainLink{},
	}

	lambdaDynamoDBTableLink, _ := testProviderImpl.Link(context.TODO(), "aws/lambda/function", "aws/dynamodb/table")
	// For fixture 3, functions do not link to tables, references are being tested for this fixture
	// so each function will have a reference to the orders table defined in the ref chain fixtures.
	getOrdersFunction := &links.ChainLink{
		ResourceName: "getOrdersFunction",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/lambda/function"},
		},
		Paths:               []string{"/orderApi"},
		LinkImplementations: map[string]provider.Link{},
		LinkedFrom: []*links.ChainLink{
			orderApi,
		},
	}
	createOrderFunction := &links.ChainLink{
		ResourceName: "createOrderFunction",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/lambda/function"},
		},
		LinkImplementations: map[string]provider.Link{},
		Paths:               []string{"/orderApi"},
		LinkedFrom: []*links.ChainLink{
			orderApi,
		},
	}
	updateOrderFunction := &links.ChainLink{
		ResourceName: "updateOrderFunction",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/lambda/function"},
		},
		LinkImplementations: map[string]provider.Link{},
		Paths:               []string{"/orderApi"},
		LinkedFrom: []*links.ChainLink{
			orderApi,
		},
	}

	dynamoDBTableStreamLink, _ := testProviderImpl.Link(context.TODO(), "aws/dynamodb/table", "aws/dynamodb/stream")
	// The only hard link in this chain is between the orders table
	// and the orders stream.
	ordersTable := &links.ChainLink{
		ResourceName: "ordersTable",
		Resource: &schema.Resource{
			Type: &schema.ResourceTypeWrapper{Value: "aws/dynamodb/table"},
		},
		LinkImplementations: map[string]provider.Link{
			"ordersStream": dynamoDBTableStreamLink,
		},
		Paths:      []string{},
		LinkedFrom: []*links.ChainLink{},
	}

	dynamoDBStreamLambdaLink, _ := testProviderImpl.Link(context.TODO(), "aws/dynamodb/stream", "aws/lambda/function")
	ordersStream := &links.ChainLink{
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
		LinkedFrom: []*links.ChainLink{
			getOrdersFunction,
			createOrderFunction,
			updateOrderFunction,
		},
		LinksTo: []*links.ChainLink{},
	}

	// Includes transitive soft circular link.
	statsAccumulatorFunction := &links.ChainLink{
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
		LinkedFrom: []*links.ChainLink{
			ordersStream,
		},
	}

	orderApi.LinksTo = []*links.ChainLink{
		getOrdersFunction,
		createOrderFunction,
		updateOrderFunction,
	}
	ordersTable.LinksTo = []*links.ChainLink{
		ordersStream,
	}
	ordersStream.LinksTo = []*links.ChainLink{
		statsAccumulatorFunction,
	}
	statsAccumulatorFunction.LinksTo = []*links.ChainLink{
		ordersTable,
	}

	return []*links.ChainLink{
		orderApi,
		ordersTable,
	}
}

func orderFixture3RefChains(
	linkChains []*links.ChainLink,
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

func TestOrderingTestSuite(t *testing.T) {
	suite.Run(t, new(OrderingTestSuite))
}

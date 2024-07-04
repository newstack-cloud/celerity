package links

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/provider"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/common/pkg/core"
	. "gopkg.in/check.v1"
)

type SpecLinkInfoTestSuite struct {
	resourceProviders map[string]provider.Provider
}

var _ = Suite(&SpecLinkInfoTestSuite{})

func (s *SpecLinkInfoTestSuite) SetUpSuite(c *C) {
	awsProvider := newTestAWSProvider()
	s.resourceProviders = map[string]provider.Provider{
		"aws/apigateway/api":         awsProvider,
		"aws/sqs/queue":              awsProvider,
		"aws/lambda/function":        awsProvider,
		"stratosaws/lambda/function": awsProvider,
		"aws/dynamodb/table":         awsProvider,
		"aws/dynamodb/stream":        awsProvider,
		"aws/iam/role":               awsProvider,
		"stratosaws/iam/role":        awsProvider,
	}
}

func (s *SpecLinkInfoTestSuite) Test_get_links_from_spec_1(c *C) {
	specLinkInfo, err := NewDefaultLinkInfoProvider(
		s.resourceProviders, &testBlueprintSpec{
			schema: testSpecLinkInfoBlueprintSchema1,
		})
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	chains, err := specLinkInfo.Links(context.Background())
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	// Using snapshots for chain links as is more convenient than building custom test functions
	// that traverse through and compare the produced chain with a "hand-crafted" expected chain.
	// Not using snapshots would require value comparisions as you traverse through the pointers in the chain.
	// However, it's really important that you take care when reviewing failing snapshot tests
	// and not just re-building the snapshots without checking the changes are correct!
	err = cupaloy.Snapshot(normaliseForSnapshot(chains, []string{}))
	if err != nil {
		c.Error(err)
	}
}

func (s *SpecLinkInfoTestSuite) Test_get_links_from_spec_2(c *C) {
	specLinkInfo, err := NewDefaultLinkInfoProvider(
		s.resourceProviders, &testBlueprintSpec{
			schema: testSpecLinkInfoBlueprintSchema2,
		})
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	chains, err := specLinkInfo.Links(context.Background())
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	// Using snapshots for chain links as is more convenient than building custom test functions
	// that traverse through and compare the produced chain with a "hand-crafted" expected chain.
	// Not using snapshots would require value comparisions as you traverse through the pointers in the chain.
	// However, it's really important that you take care when reviewing failing snapshot tests
	// and not just re-building the snapshots without checking the changes are correct!
	err = cupaloy.Snapshot(normaliseForSnapshot(chains, []string{}))
	if err != nil {
		c.Error(err)
	}
}

func (s *SpecLinkInfoTestSuite) Test_get_links_from_spec_for_a_blueprint_with_circular_soft_links(c *C) {
	specLinkInfo, err := NewDefaultLinkInfoProvider(
		s.resourceProviders, &testBlueprintSpec{
			schema: testSpecLinkInfoBlueprintSchema5,
		})
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	chains, err := specLinkInfo.Links(context.Background())
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	// We can't use cupaloy.Snapshot to take snapshots for blueprints with circular soft links
	// as the link that appears at the start of the chain is not deterministic.
	err = circularLinksApproximateSnapshotSchema5(normaliseForSnapshot(chains, []string{}))
	if err != nil {
		c.Error(err)
	}
}

func (s *SpecLinkInfoTestSuite) Test_get_links_fails_when_a_link_implementation_does_not_exist_for_linked_resources(c *C) {
	specLinkInfo, err := NewDefaultLinkInfoProvider(
		s.resourceProviders, &testBlueprintSpec{
			schema: testSpecLinkInfoBlueprintSchema3,
		})
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	_, err = specLinkInfo.Links(context.Background())
	if err == nil {
		c.Error("expected an error for missing link implementation")
		c.FailNow()
	}
	linkError, isLinkError := err.(*LinkError)
	if !isLinkError {
		c.Error("expected error to be an instance of a LinkError")
		c.FailNow()
	}

	if linkError.ReasonCode != LinkErrorReasonCodeMissingLinkImpl {
		c.Errorf(
			"expected link error reason code to be %s, found %s",
			LinkErrorReasonCodeMissingLinkImpl,
			linkError.ReasonCode,
		)
	}
}

func (s *SpecLinkInfoTestSuite) Test_get_links_fails_when_circular_hard_links_are_discovered(c *C) {
	specLinkInfo, err := NewDefaultLinkInfoProvider(
		s.resourceProviders, &testBlueprintSpec{
			schema: testSpecLinkInfoBlueprintSchema4,
		})
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	_, err = specLinkInfo.Links(context.Background())
	if err == nil {
		c.Error("expected an error for circular hard links")
		c.FailNow()
	}
	linkError, isLinkError := err.(*LinkError)
	if !isLinkError {
		c.Error("expected error to be an instance of a LinkError")
		c.FailNow()
	}

	if linkError.ReasonCode != LinkErrorReasonCodeCircularLinks {
		c.Errorf(
			"expected link error reason code to be %s, found %s",
			LinkErrorReasonCodeCircularLinks,
			linkError.ReasonCode,
		)
		c.FailNow()
	}

	if len(linkError.ChildErrors) != 2 {
		c.Errorf("expected 2 circular hard link child errors, found %d", len(linkError.ChildErrors))
		c.FailNow()
	}

	for i := 0; i < 2; i += 1 {
		childLinkError, isChildErrorLinkError := linkError.ChildErrors[i].(*LinkError)
		if !isChildErrorLinkError {
			c.Errorf("expected child error %d to be an instance of a LinkError", i+1)
			c.FailNow()
		}

		if childLinkError.ReasonCode != LinkErrorReasonCodeCircularLink {
			c.Errorf(
				"expected child link error %d reason code to be %s, found %s",
				i+1,
				LinkErrorReasonCodeCircularLink,
				linkError.ReasonCode,
			)
			c.FailNow()
		}
	}
}

func (s *SpecLinkInfoTestSuite) Test_get_link_warnings_from_spec_for_a_blueprint_with_non_common_terminals(c *C) {
	// Re-use schema fixture 1 as it has the standalone IAM role
	// and the IAM role resource type is not a common terminal.
	specLinkInfo, err := NewDefaultLinkInfoProvider(
		s.resourceProviders, &testBlueprintSpec{
			schema: testSpecLinkInfoBlueprintSchema1,
		})
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	warnings, err := specLinkInfo.Warnings(context.Background())
	if err != nil {
		c.Error(err)
		c.FailNow()
	}
	c.Assert(warnings, DeepEquals, []string{
		"resource \"statsAccumulatorFunction\" of type \"aws/lambda/function\" does not link out to " +
			"any other resources where in most use-cases a resource of type \"aws/lambda/function\" is expected to link to other resources",
		"resource \"standaloneRole\" of type \"aws/iam/role\" does not link out to any other resources " +
			"where in most use-cases a resource of type \"aws/iam/role\" is expected to link to other resources",
	})
}

// Acts as a normaliser as ordering does not matter in chain links but does matter when comparing
// snapshots!
// Also, to simplify the structure that is snapshotted we will convert linked from references to strings
// containing resource names. This will resolve false negatives in snapshot failure and make it easier to read
// the snapshots.
func normaliseForSnapshot(chains []*ChainLink, ancestors []string) []*snapshotChainLink {
	orderedChainLinks := append([]*ChainLink{}, chains...)
	sort.SliceStable(orderedChainLinks, func(i, j int) bool {
		return orderedChainLinks[i].ResourceName < orderedChainLinks[j].ResourceName
	})
	ssChainLinks := []*snapshotChainLink{}
	for _, chainLink := range orderedChainLinks {

		ssChainLink := &snapshotChainLink{
			ResourceName:        chainLink.ResourceName,
			Selectors:           chainLink.Selectors,
			LinkImplementations: chainLink.LinkImplementations,
			Resource:            chainLink.Resource,
			Paths:               chainLink.Paths,
		}
		sort.Strings(ssChainLink.Paths)
		// Prevent infinite loops/stack overflows when normalising chains
		// with circular links.
		if !core.SliceContainsComparable(ancestors, chainLink.ResourceName) {
			ssChainLink.LinksTo = normaliseForSnapshot(chainLink.LinksTo, append(ancestors, chainLink.ResourceName))
		} else {
			// To avoid going in circles, we'll create copies of the linked to
			// items only containing resource names for the purpose of snapshots.
			ssChainLink.LinksTo = createCycleStubsForSnapshot(chainLink.LinksTo)
		}
		ssChainLink.LinkedFrom = core.Map(
			chainLink.LinkedFrom,
			func(linkedFrom *ChainLink, _ int) string {
				return linkedFrom.ResourceName
			},
		)
		sort.Strings(ssChainLink.LinkedFrom)
		for selectorKey, selectedResources := range chainLink.Selectors {
			sort.Strings(selectedResources)
			chainLink.Selectors[selectorKey] = selectedResources
		}
		ssChainLinks = append(ssChainLinks, ssChainLink)
	}

	return ssChainLinks
}

func createCycleStubsForSnapshot(links []*ChainLink) []*snapshotChainLink {
	cycleStubs := []*snapshotChainLink{}
	for _, link := range links {
		cycleStubs = append(cycleStubs, &snapshotChainLink{
			ResourceName: fmt.Sprintf("%s-cycleStub", link.ResourceName),
		})
	}
	return cycleStubs
}

func circularLinksApproximateSnapshotSchema5(ssChainLinks []*snapshotChainLink) error {
	possiblePaths := map[string]interface{}{
		"statsRetrieverFunction": map[string]interface{}{
			"lambdaExecutionRole": map[string]interface{}{
				"statsRetrieverFunction": "lambdaExecutionRole-cycleStub",
			},
		},
		"lambdaExecutionRole": map[string]interface{}{
			"statsRetrieverFunction": map[string]interface{}{
				"lambdaExecutionRole": "statsRetrieverFunction-cycleStub",
			},
		},
		"ordersTable": map[string]interface{}{
			"ordersStream": map[string]interface{}{
				"statsAccumulatorFunction": map[string]interface{}{
					"ordersTable": "ordersStream-cycleStub",
				},
			},
		},
		"ordersStream": map[string]interface{}{
			"statsAccumulatorFunction": map[string]interface{}{
				"ordersTable": map[string]interface{}{
					"ordersStream": "statsAccumulatorFunction-cycleStub",
				},
			},
		},
		"statsAccumulatorFunction": map[string]interface{}{
			"ordersTable": map[string]interface{}{
				"ordersStream": map[string]interface{}{
					"statsAccumulatorFunction": "ordersTable-cycleStub",
				},
			},
		},
	}

	if len(ssChainLinks) != 2 {
		return fmt.Errorf("expected 2 top-level chains, found %d", len(ssChainLinks))
	}

	for _, ssChainLink := range ssChainLinks {
		chainLinkError := followPaths(ssChainLink, possiblePaths)
		if chainLinkError != nil {
			return chainLinkError
		}
	}

	return nil
}

func followPaths(ssChainLink *snapshotChainLink, possiblePaths interface{}) error {
	possiblePathsStr, isTerminal := possiblePaths.(string)
	if isTerminal {
		if ssChainLink.ResourceName != possiblePathsStr {
			return fmt.Errorf(
				"%s is not the expected cycle stub \"%s\"",
				ssChainLink.ResourceName,
				possiblePathsStr,
			)
		}
		return nil
	}

	mapping, isMap := possiblePaths.(map[string]interface{})
	if isMap {
		nextLevelPath, hasNextLevelPath := mapping[ssChainLink.ResourceName]
		if !hasNextLevelPath {
			return fmt.Errorf("%s is not an expected resource in possible paths", ssChainLink.ResourceName)
		}
		return followPaths(ssChainLink.LinksTo[0], nextLevelPath)
	}

	return errors.New("unexpected possiblePaths type provided")
}

// Lots of links.
var testSpecLinkInfoBlueprintSchema1 = &schema.Blueprint{
	Resources: &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"orderApi": {
				Type: "aws/apigateway/api",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"app": "orderApi",
						},
					},
				},
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"app": "orderApi",
						},
					},
				},
			},
			"orderQueue": {
				Type: "aws/sqs/queue",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"app": "orderWorkflow",
						},
					},
				},
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"app": "orderWorkflow",
						},
					},
				},
			},
			"processOrdersFunction": {
				Type: "aws/lambda/function",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"app": "orderWorkflow",
						},
					},
				},
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"system": "orders",
						},
					},
				},
			},
			"createOrderFunction": {
				Type: "aws/lambda/function",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"app": "orderApi",
						},
					},
				},
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"system": "orders",
						},
					},
				},
			},
			"getOrdersFunction": {
				Type: "aws/lambda/function",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"app": "orderApi",
						},
					},
				},
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"system": "orders",
						},
					},
				},
			},
			"ordersTable": {
				Type: "aws/dynamodb/table",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"system": "orders",
						},
					},
				},
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"system": "orders",
						},
					},
				},
			},
			"ordersStream": {
				Type: "aws/dynamodb/stream",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"system": "orders",
						},
					},
				},
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"workflow": "orderStats",
						},
					},
				},
			},
			"statsAccumulatorFunction": {
				Type: "aws/lambda/function",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"workflow": "orderStats",
						},
					},
				},
			},
			"standaloneRole": {
				Type:     "aws/iam/role",
				Metadata: &schema.Metadata{},
			},
		},
	},
}

// No links.
var testSpecLinkInfoBlueprintSchema2 = &schema.Blueprint{
	Resources: &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"exchangeRateFunction": {
				Type: "aws/lambda/function",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"app": "exchangeRates",
						},
					},
				},
			},
			"refreshExchangeRatesFunction": {
				Type: "aws/lambda/function",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"app": "exchangeRates",
						},
					},
				},
			},
			"standaloneRole2": {
				Type:     "aws/iam/role",
				Metadata: &schema.Metadata{},
			},
		},
	},
}

// Missing link implementation.
// A lambda can link to another lambda as per bootstrap_test.go
// fixture set up, however there is no link implementation for
// lambda to lambda links.
var testSpecLinkInfoBlueprintSchema3 = &schema.Blueprint{
	Resources: &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"exchangeRatesFunction": {
				Type: "aws/lambda/function",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"app": "exchangeRates",
						},
					},
				},
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"app": "exchangeRates",
						},
					},
				},
			},
			"saveExchangeRatesFunction": {
				Type: "aws/lambda/function",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"app": "exchangeRates",
						},
					},
				},
			},
		},
	},
}

// Circular hard links.
// As soon as the first circular hard link is found in a chain, no further links
// in that chain are discovered until the first one is fixed!
// The test cases to capture multiple circular hard links here are for independent chains.
// Soft links break hard link cycles as soft links represent dependencies where one resource
// does not need to exist in order to deploy/create the other.
// For this blueprint, an error should be returned.
var testSpecLinkInfoBlueprintSchema4 = &schema.Blueprint{
	Resources: &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"ordersTable": {
				Type: "aws/dynamodb/table",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"system": "orders",
						},
					},
				},
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"system": "orders",
						},
					},
				},
			},
			"ordersStream": {
				Type: "aws/dynamodb/stream",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"system": "orders",
						},
					},
				},
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"workflow": "orderStats",
						},
					},
				},
			},
			"statsAccumulatorFunction": {
				Type: "aws/lambda/function",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"workflow": "orderStats",
						},
					},
				},
				// Indirect hard circular link back to orders table.
				// (In reality the relationship between an lambda and a DynamoDB table is
				// not hard but for the sake of this test case it is)
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"system": "orders",
						},
					},
				},
			},
			"statsRetrieverFunction": {
				Type: "aws/lambda/function",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"purpose": "retrieveStats",
						},
					},
				},
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"purpose": "retrieveStats",
						},
					},
				},
			},
			"lambdaExecutionRole": {
				Type: "aws/iam/role",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"purpose": "retrieveStats",
						},
					},
				},
				// Direct hard circular link between statsRetrieverFunction
				// and lambdaExecutionRole.
				// (In reality the relationship between an IAM role and a lambda is
				// not hard but for the sake of this test case it is)
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"purpose": "retrieveStats",
						},
					},
				},
			},
		},
	},
}

// Circular links with soft links breaking hard link cycles.
// This should not cause an error!
var testSpecLinkInfoBlueprintSchema5 = &schema.Blueprint{
	Resources: &schema.ResourceMap{
		Values: map[string]*schema.Resource{
			"ordersTable": {
				Type: "aws/dynamodb/table",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"system": "orders",
						},
					},
				},
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"system": "orders",
						},
					},
				},
			},
			"ordersStream": {
				Type: "aws/dynamodb/stream",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"system": "orders",
						},
					},
				},
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"workflow": "orderStats",
						},
					},
				},
			},
			"statsAccumulatorFunction": {
				// Represents a theoretical stratos abstraction
				// of an aws lambda function.
				Type: "stratosaws/lambda/function",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"workflow": "orderStats",
						},
					},
				},
				// Indirect circular link back to orders table.
				// The soft link between "stratosaws/lambda/function"
				// and "aws/dynamodb/table" breaks the hard link cycle.
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"system": "orders",
						},
					},
				},
			},
			"statsRetrieverFunction": {
				Type: "aws/lambda/function",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"purpose": "retrieveStats",
						},
					},
				},
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"purpose": "retrieveStats",
						},
					},
				},
			},
			"lambdaExecutionRole": {
				// Represents a theoretical stratos abstraction
				// of an aws iam role.
				Type: "stratosaws/iam/role",
				Metadata: &schema.Metadata{
					Labels: &schema.StringMap{
						Values: map[string]string{
							"purpose": "retrieveStats",
						},
					},
				},
				// Direct soft circular link between statsRetrieverFunction
				// and lambdaExecutionRole.
				LinkSelector: &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{
							"purpose": "retrieveStats",
						},
					},
				},
			},
		},
	},
}

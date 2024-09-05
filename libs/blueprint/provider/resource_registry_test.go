package provider

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/errors"
	. "gopkg.in/check.v1"
)

type ResourceRegistryTestSuite struct {
	resourceRegistry ResourceRegistry
	testResource     *testExampleResource
}

var _ = Suite(&ResourceRegistryTestSuite{})

func (s *ResourceRegistryTestSuite) SetUpTest(c *C) {
	testRes := newTestExampleResource()

	providers := map[string]Provider{
		"test": &testProvider{
			resources: map[string]Resource{
				"test/exampleResource": testRes,
			},
			namespace: "test",
		},
	}

	s.testResource = testRes.(*testExampleResource)
	s.resourceRegistry = NewResourceRegistry(providers)
}

func (s *ResourceRegistryTestSuite) Test_get_definition(c *C) {
	output, err := s.resourceRegistry.GetSpecDefinition(
		context.TODO(),
		"test/exampleResource",
		&ResourceGetSpecDefinitionInput{},
	)
	c.Assert(err, IsNil)
	c.Assert(output.SpecDefinition, DeepEquals, s.testResource.definition)

	// Second time should be cached and produce the same result.
	output, err = s.resourceRegistry.GetSpecDefinition(
		context.TODO(),
		"test/exampleResource",
		&ResourceGetSpecDefinitionInput{},
	)
	c.Assert(err, IsNil)
	c.Assert(output.SpecDefinition, DeepEquals, s.testResource.definition)
}

func (s *ResourceRegistryTestSuite) Test_has_resource_type(c *C) {
	hasResourceType, err := s.resourceRegistry.HasResourceType(context.TODO(), "test/exampleResource")
	c.Assert(err, IsNil)
	c.Assert(hasResourceType, Equals, true)

	hasResourceType, err = s.resourceRegistry.HasResourceType(context.TODO(), "test/otherResource")
	c.Assert(err, IsNil)
	c.Assert(hasResourceType, Equals, false)
}

func (s *ResourceRegistryTestSuite) Test_produces_error_for_missing_provider(c *C) {
	_, err := s.resourceRegistry.HasResourceType(context.TODO(), "otherProvider/otherResource")
	c.Assert(err, NotNil)
	runErr, isRunErr := err.(*errors.RunError)
	c.Assert(isRunErr, Equals, true)
	c.Assert(runErr.ReasonCode, Equals, ErrorReasonCodeResourceTypeProviderNotFound)
	c.Assert(runErr.Error(), Equals, "run error: run failed as the provider with namespace \"otherProvider\" "+
		"was not found for resource type \"otherProvider/otherResource\"")
}

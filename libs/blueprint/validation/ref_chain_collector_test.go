package validation

import (
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	. "gopkg.in/check.v1"
)

type RefChainCollectorTestSuite struct {
	resourceA *schema.Resource
	resourceB *schema.Resource
	resourceC *schema.Resource
	resourceD *schema.Resource
	resourceE *schema.Resource
	resourceF *schema.Resource
}

var _ = Suite(&RefChainCollectorTestSuite{})

func (s *RefChainCollectorTestSuite) SetUpTest(c *C) {
	s.resourceA = createTestResource("resourceA")
	s.resourceB = createTestResource("resourceB")
	s.resourceC = createTestResource("resourceC")
	s.resourceD = createTestResource("resourceD")
	s.resourceE = createTestResource("resourceE")
	s.resourceF = createTestResource("resourceF")
}

func (s *RefChainCollectorTestSuite) Test_detects_circular_references(c *C) {

	collector := NewRefChainCollector()
	err := collector.Collect("resources.resourceA", s.resourceA, "")
	if err != nil {
		c.Fatal(err)
	}
	err = collector.Collect("resources.resourceB", s.resourceB, "resources.resourceA")
	if err != nil {
		c.Fatal(err)
	}
	err = collector.Collect("resources.resourceC", s.resourceC, "resources.resourceB")
	if err != nil {
		c.Fatal(err)
	}
	err = collector.Collect("resources.resourceA", s.resourceA, "resources.resourceC")
	if err != nil {
		c.Fatal(err)
	}

	err = collector.Collect("resources.resourceD", s.resourceD, "")
	if err != nil {
		c.Fatal(err)
	}
	err = collector.Collect("resources.resourceE", s.resourceE, "resources.resourceD")
	if err != nil {
		c.Fatal(err)
	}
	err = collector.Collect("resources.resourceF", s.resourceF, "resources.resourceE")
	if err != nil {
		c.Fatal(err)
	}
	err = collector.Collect("resources.resourceD", s.resourceD, "resources.resourceF")
	if err != nil {
		c.Fatal(err)
	}

	circularRefs := collector.FindCircularReferences()
	c.Assert(circularRefs, HasLen, 2)
	c.Assert(circularRefs[0].ElementName, Equals, "resources.resourceA")
	c.Assert(circularRefs[1].ElementName, Equals, "resources.resourceD")
}

func (s *RefChainCollectorTestSuite) Test_finds_no_circular_references(c *C) {
	collector := NewRefChainCollector()
	err := collector.Collect("resources.resourceA", s.resourceA, "")
	if err != nil {
		c.Fatal(err)
	}
	err = collector.Collect("resources.resourceB", s.resourceB, "resources.resourceA")
	if err != nil {
		c.Fatal(err)
	}
	err = collector.Collect("resources.resourceC", s.resourceC, "resources.resourceB")
	if err != nil {
		c.Fatal(err)
	}

	err = collector.Collect("resources.resourceD", s.resourceD, "")
	if err != nil {
		c.Fatal(err)
	}
	err = collector.Collect("resources.resourceE", s.resourceE, "resources.resourceD")
	if err != nil {
		c.Fatal(err)
	}
	err = collector.Collect("resources.resourceF", s.resourceF, "resources.resourceE")
	if err != nil {
		c.Fatal(err)
	}

	circularRefs := collector.FindCircularReferences()
	c.Assert(circularRefs, HasLen, 0)
}

func createTestResource(id string) *schema.Resource {
	return &schema.Resource{
		Type: &schema.ResourceTypeWrapper{Value: "celerity/example"},
		Spec: &core.MappingNode{
			Fields: map[string]*core.MappingNode{
				"id": {
					Literal: &core.ScalarValue{
						StringValue: &id,
					},
				},
			},
		},
	}
}

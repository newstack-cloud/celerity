package provider

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/errors"
	. "gopkg.in/check.v1"
)

type DataSourceRegistryTestSuite struct {
	dataSourceRegistry DataSourceRegistry
	testDataSource     *testExampleDataSource
}

var _ = Suite(&DataSourceRegistryTestSuite{})

func (s *DataSourceRegistryTestSuite) SetUpTest(c *C) {
	testDataSource := newTestExampleDataSource()

	providers := map[string]Provider{
		"test": &testProvider{
			dataSources: map[string]DataSource{
				"test/exampleDataSource": testDataSource,
			},
			namespace: "test",
		},
	}

	s.testDataSource = testDataSource.(*testExampleDataSource)
	s.dataSourceRegistry = NewDataSourceRegistry(providers)
}

func (s *DataSourceRegistryTestSuite) Test_get_definition(c *C) {
	output, err := s.dataSourceRegistry.GetSpecDefinition(
		context.TODO(),
		"test/exampleDataSource",
		&DataSourceGetSpecDefinitionInput{},
	)
	c.Assert(err, IsNil)
	c.Assert(output.SpecDefinition, DeepEquals, s.testDataSource.definition)

	// Second time should be cached and produce the same result.
	output, err = s.dataSourceRegistry.GetSpecDefinition(
		context.TODO(),
		"test/exampleDataSource",
		&DataSourceGetSpecDefinitionInput{},
	)
	c.Assert(err, IsNil)
	c.Assert(output.SpecDefinition, DeepEquals, s.testDataSource.definition)
}

func (s *DataSourceRegistryTestSuite) Test_has_resource_type(c *C) {
	hasDSType, err := s.dataSourceRegistry.HasDataSourceType(context.TODO(), "test/exampleDataSource")
	c.Assert(err, IsNil)
	c.Assert(hasDSType, Equals, true)

	hasDSType, err = s.dataSourceRegistry.HasDataSourceType(context.TODO(), "test/otherDataSource")
	c.Assert(err, IsNil)
	c.Assert(hasDSType, Equals, false)
}

func (s *DataSourceRegistryTestSuite) Test_produces_error_for_missing_provider(c *C) {
	_, err := s.dataSourceRegistry.HasDataSourceType(context.TODO(), "otherProvider/otherDataSource")
	c.Assert(err, NotNil)
	runErr, isRunErr := err.(*errors.RunError)
	c.Assert(isRunErr, Equals, true)
	c.Assert(runErr.ReasonCode, Equals, ErrorReasonCodeItemTypeProviderNotFound)
	c.Assert(runErr.Error(), Equals, "run error: run failed as the provider with namespace \"otherProvider\" "+
		"was not found for data source type \"otherProvider/otherDataSource\"")
}

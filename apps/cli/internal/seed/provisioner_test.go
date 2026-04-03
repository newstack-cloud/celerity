package seed

import (
	"context"
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type ProvisionerTestSuite struct {
	suite.Suite
	logger *zap.Logger
}

func TestProvisionerTestSuite(t *testing.T) {
	suite.Run(t, new(ProvisionerTestSuite))
}

func (s *ProvisionerTestSuite) SetupTest() {
	logger, _ := zap.NewDevelopment()
	s.logger = logger
}

func (s *ProvisionerTestSuite) loadBlueprint(yamlContent string) *schema.Blueprint {
	bp, err := schema.LoadString(yamlContent, schema.YAMLSpecFormat)
	s.Require().NoError(err, "failed to load test blueprint")
	return bp
}

type mockDSProvisioner struct {
	tables []TableDefinition
	err    error
}

func (m *mockDSProvisioner) ProvisionTable(_ context.Context, table TableDefinition) error {
	m.tables = append(m.tables, table)
	return m.err
}

type mockBucketProvisioner struct {
	buckets []string
	err     error
}

func (m *mockBucketProvisioner) ProvisionBucket(_ context.Context, bucketName string) error {
	m.buckets = append(m.buckets, bucketName)
	return m.err
}

func (s *ProvisionerTestSuite) Test_nil_resources_returns_empty_result() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources: {}
`)
	ds := &mockDSProvisioner{}
	bk := &mockBucketProvisioner{}
	result, err := ProvisionFromBlueprint(context.Background(), bp, ds, bk, nil, s.logger)
	s.Require().NoError(err)
	s.Assert().Empty(result.Tables)
	s.Assert().Empty(result.Buckets)
}

func (s *ProvisionerTestSuite) Test_provisions_datastore_table() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
      keys:
        partitionKey: userId
      schema:
        fields:
          userId: string
          email: string
`)
	ds := &mockDSProvisioner{}
	bk := &mockBucketProvisioner{}
	result, err := ProvisionFromBlueprint(context.Background(), bp, ds, bk, nil, s.logger)
	s.Require().NoError(err)
	s.Require().Len(result.Tables, 1)
	s.Assert().Equal("users", result.Tables[0])
	s.Require().Len(ds.tables, 1)
	s.Assert().Equal("users", ds.tables[0].Name)
	s.Assert().Equal("userId", ds.tables[0].PartitionKey.Name)
	s.Assert().Equal("string", ds.tables[0].PartitionKey.DataType)
}

func (s *ProvisionerTestSuite) Test_provisions_table_with_sort_key() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  ordersTable:
    type: "celerity/datastore"
    spec:
      name: orders
      keys:
        partitionKey: customerId
        sortKey: orderDate
      schema:
        fields:
          customerId: string
          orderDate: string
`)
	ds := &mockDSProvisioner{}
	result, err := ProvisionFromBlueprint(context.Background(), bp, ds, &mockBucketProvisioner{}, nil, s.logger)
	s.Require().NoError(err)
	s.Require().Len(result.Tables, 1)
	s.Require().NotNil(ds.tables[0].SortKey)
	s.Assert().Equal("orderDate", ds.tables[0].SortKey.Name)
}

func (s *ProvisionerTestSuite) Test_provisions_table_with_stream_enabled() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
      keys:
        partitionKey: userId
      schema:
        fields:
          userId: string
`)
	ds := &mockDSProvisioner{}
	streamEnabled := map[string]bool{"usersTable": true}
	_, err := ProvisionFromBlueprint(context.Background(), bp, ds, &mockBucketProvisioner{}, streamEnabled, s.logger)
	s.Require().NoError(err)
	s.Require().Len(ds.tables, 1)
	s.Assert().True(ds.tables[0].StreamEnabled)
}

func (s *ProvisionerTestSuite) Test_provisions_bucket() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  assetsBucket:
    type: "celerity/bucket"
    spec:
      name: my-assets
`)
	ds := &mockDSProvisioner{}
	bk := &mockBucketProvisioner{}
	result, err := ProvisionFromBlueprint(context.Background(), bp, ds, bk, nil, s.logger)
	s.Require().NoError(err)
	s.Require().Len(result.Buckets, 1)
	s.Assert().Equal("my-assets", result.Buckets[0])
	s.Require().Len(bk.buckets, 1)
	s.Assert().Equal("my-assets", bk.buckets[0])
}

func (s *ProvisionerTestSuite) Test_bucket_name_defaults_to_resource_name() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  assetsBucket:
    type: "celerity/bucket"
    spec: {}
`)
	bk := &mockBucketProvisioner{}
	result, err := ProvisionFromBlueprint(context.Background(), bp, &mockDSProvisioner{}, bk, nil, s.logger)
	s.Require().NoError(err)
	s.Assert().Equal("assetsBucket", result.Buckets[0])
}

func (s *ProvisionerTestSuite) Test_datastore_provisioner_error_propagates() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
      keys:
        partitionKey: userId
      schema:
        fields:
          userId: string
`)
	ds := &mockDSProvisioner{err: context.DeadlineExceeded}
	_, err := ProvisionFromBlueprint(context.Background(), bp, ds, &mockBucketProvisioner{}, nil, s.logger)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "provisioning table")
}

func (s *ProvisionerTestSuite) Test_bucket_provisioner_error_propagates() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  assetsBucket:
    type: "celerity/bucket"
    spec:
      name: assets
`)
	bk := &mockBucketProvisioner{err: context.DeadlineExceeded}
	_, err := ProvisionFromBlueprint(context.Background(), bp, &mockDSProvisioner{}, bk, nil, s.logger)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "provisioning bucket")
}

func (s *ProvisionerTestSuite) Test_mixed_resources() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersTable:
    type: "celerity/datastore"
    spec:
      name: users
      keys:
        partitionKey: userId
      schema:
        fields:
          userId: string
  assetsBucket:
    type: "celerity/bucket"
    spec:
      name: assets
  myApi:
    type: "celerity/api"
    spec:
      protocols:
        - http
`)
	ds := &mockDSProvisioner{}
	bk := &mockBucketProvisioner{}
	result, err := ProvisionFromBlueprint(context.Background(), bp, ds, bk, nil, s.logger)
	s.Require().NoError(err)
	s.Assert().Len(result.Tables, 1)
	s.Assert().Len(result.Buckets, 1)
}

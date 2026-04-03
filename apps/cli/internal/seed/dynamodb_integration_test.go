package seed

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/newstack-cloud/celerity/apps/cli/internal/testutils"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type DynamoDBIntegrationSuite struct {
	suite.Suite
	endpoint string
	logger   *zap.Logger
}

func TestDynamoDBIntegrationSuite(t *testing.T) {
	suite.Run(t, new(DynamoDBIntegrationSuite))
}

func (s *DynamoDBIntegrationSuite) SetupTest() {
	s.endpoint = testutils.RequireEnv(s.T(), "CELERITY_TEST_DYNAMODB_ENDPOINT")
	logger, _ := zap.NewDevelopment()
	s.logger = logger
}

func (s *DynamoDBIntegrationSuite) newClient() *dynamodb.Client {
	return dynamodb.New(dynamodb.Options{
		BaseEndpoint: aws.String(s.endpoint),
		Region:       "us-east-1",
		Credentials:  credentials.NewStaticCredentialsProvider("local", "local", ""),
	})
}

func (s *DynamoDBIntegrationSuite) deleteTable(tableName string) {
	client := s.newClient()
	_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
		TableName: aws.String(tableName),
	})
}

func (s *DynamoDBIntegrationSuite) Test_provision_table_with_partition_key_only() {
	tableName := "integration_test_pk_only"
	s.deleteTable(tableName)

	provisioner := NewDynamoDBProvisioner(s.endpoint, s.logger)
	err := provisioner.ProvisionTable(context.Background(), TableDefinition{
		Name:         tableName,
		PartitionKey: KeyField{Name: "id", DataType: "string"},
	})
	s.Require().NoError(err)

	// Verify table exists via describe.
	client := s.newClient()
	desc, err := client.DescribeTable(context.Background(), &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	s.Require().NoError(err)
	s.Assert().Equal(tableName, *desc.Table.TableName)
}

func (s *DynamoDBIntegrationSuite) Test_provision_table_with_sort_key() {
	tableName := "integration_test_pk_sk"
	s.deleteTable(tableName)

	provisioner := NewDynamoDBProvisioner(s.endpoint, s.logger)
	err := provisioner.ProvisionTable(context.Background(), TableDefinition{
		Name:         tableName,
		PartitionKey: KeyField{Name: "pk", DataType: "string"},
		SortKey:      &KeyField{Name: "sk", DataType: "string"},
	})
	s.Require().NoError(err)

	client := s.newClient()
	desc, err := client.DescribeTable(context.Background(), &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	s.Require().NoError(err)
	s.Assert().Len(desc.Table.KeySchema, 2)
}

func (s *DynamoDBIntegrationSuite) Test_provision_table_with_gsi() {
	tableName := "integration_test_gsi"
	s.deleteTable(tableName)

	provisioner := NewDynamoDBProvisioner(s.endpoint, s.logger)
	err := provisioner.ProvisionTable(context.Background(), TableDefinition{
		Name:         tableName,
		PartitionKey: KeyField{Name: "id", DataType: "string"},
		Indexes: []IndexDefinition{
			{
				Name:   "email-index",
				Fields: []KeyField{{Name: "email", DataType: "string"}},
			},
		},
	})
	s.Require().NoError(err)

	client := s.newClient()
	desc, err := client.DescribeTable(context.Background(), &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	s.Require().NoError(err)
	s.Require().Len(desc.Table.GlobalSecondaryIndexes, 1)
	s.Assert().Equal("email-index", *desc.Table.GlobalSecondaryIndexes[0].IndexName)
}

func (s *DynamoDBIntegrationSuite) Test_provision_table_with_stream() {
	tableName := "integration_test_stream"
	s.deleteTable(tableName)

	provisioner := NewDynamoDBProvisioner(s.endpoint, s.logger)
	err := provisioner.ProvisionTable(context.Background(), TableDefinition{
		Name:          tableName,
		PartitionKey:  KeyField{Name: "id", DataType: "string"},
		StreamEnabled: true,
	})
	s.Require().NoError(err)

	client := s.newClient()
	desc, err := client.DescribeTable(context.Background(), &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	s.Require().NoError(err)
	s.Require().NotNil(desc.Table.StreamSpecification)
	s.Assert().True(*desc.Table.StreamSpecification.StreamEnabled)
}

func (s *DynamoDBIntegrationSuite) Test_provision_duplicate_table_is_idempotent() {
	tableName := "integration_test_idempotent"
	s.deleteTable(tableName)

	provisioner := NewDynamoDBProvisioner(s.endpoint, s.logger)
	td := TableDefinition{
		Name:         tableName,
		PartitionKey: KeyField{Name: "id", DataType: "string"},
	}

	err := provisioner.ProvisionTable(context.Background(), td)
	s.Require().NoError(err)

	// Second call should succeed (table already exists).
	err = provisioner.ProvisionTable(context.Background(), td)
	s.Require().NoError(err)
}

// --- DynamoDBSeeder ---

func (s *DynamoDBIntegrationSuite) Test_put_item_and_read_back() {
	tableName := "integration_test_seeder"
	s.deleteTable(tableName)

	provisioner := NewDynamoDBProvisioner(s.endpoint, s.logger)
	s.Require().NoError(provisioner.ProvisionTable(context.Background(), TableDefinition{
		Name:         tableName,
		PartitionKey: KeyField{Name: "id", DataType: "string"},
	}))

	seeder := NewDynamoDBSeeder(s.endpoint, s.logger)
	err := seeder.PutItem(context.Background(), tableName, []byte(`{"id":"user-1","name":"Alice","age":30}`))
	s.Require().NoError(err)

	// Read back via Scan to verify the item was written.
	client := s.newClient()
	result, err := client.Scan(context.Background(), &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	s.Require().NoError(err)
	s.Assert().Equal(int32(1), result.Count)
}

func (s *DynamoDBIntegrationSuite) Test_put_item_with_nested_types() {
	tableName := "integration_test_nested"
	s.deleteTable(tableName)

	provisioner := NewDynamoDBProvisioner(s.endpoint, s.logger)
	s.Require().NoError(provisioner.ProvisionTable(context.Background(), TableDefinition{
		Name:         tableName,
		PartitionKey: KeyField{Name: "id", DataType: "string"},
	}))

	seeder := NewDynamoDBSeeder(s.endpoint, s.logger)
	item := `{
		"id": "item-1",
		"tags": ["a", "b"],
		"metadata": {"key": "value"},
		"active": true,
		"score": 99.5,
		"deleted": null
	}`
	err := seeder.PutItem(context.Background(), tableName, []byte(item))
	s.Require().NoError(err)
}

func (s *DynamoDBIntegrationSuite) Test_put_item_invalid_json_returns_error() {
	seeder := NewDynamoDBSeeder(s.endpoint, s.logger)
	err := seeder.PutItem(context.Background(), "whatever", []byte(`{invalid`))
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "parsing item JSON")
}

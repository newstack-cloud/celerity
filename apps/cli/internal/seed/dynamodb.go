package seed

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamotypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/newstack-cloud/celerity/apps/cli/internal/seed/dynamoconv"
	"go.uber.org/zap"
)

// DynamoDBProvisioner provisions DynamoDB tables in DynamoDB Local.
type DynamoDBProvisioner struct {
	client *dynamodb.Client
	logger *zap.Logger
}

// NewDynamoDBProvisioner creates a provisioner targeting a DynamoDB Local endpoint.
func NewDynamoDBProvisioner(endpoint string, logger *zap.Logger) *DynamoDBProvisioner {
	client := dynamodb.New(dynamodb.Options{
		BaseEndpoint: aws.String(endpoint),
		Region:       "us-east-1",
		Credentials:  credentials.NewStaticCredentialsProvider("local", "local", ""),
	})

	return &DynamoDBProvisioner{
		client: client,
		logger: logger,
	}
}

func (p *DynamoDBProvisioner) ProvisionTable(ctx context.Context, table TableDefinition) error {
	input := dynamoconv.BuildCreateTableInput(toTableInput(table))

	_, err := p.client.CreateTable(ctx, input)
	if err != nil {
		if isTableAlreadyExists(err) {
			p.logger.Debug("table already exists, skipping", zap.String("table", table.Name))
			return nil
		}
		return fmt.Errorf("creating DynamoDB table %s: %w", table.Name, err)
	}

	p.logger.Debug("table created", zap.String("table", table.Name))
	return nil
}

func isTableAlreadyExists(err error) bool {
	var riuErr *dynamotypes.ResourceInUseException
	return errors.As(err, &riuErr)
}

// DynamoDBSeeder implements NoSQLSeeder for DynamoDB Local.
type DynamoDBSeeder struct {
	client *dynamodb.Client
	logger *zap.Logger
}

// NewDynamoDBSeeder creates a seeder targeting a DynamoDB Local endpoint.
func NewDynamoDBSeeder(endpoint string, logger *zap.Logger) *DynamoDBSeeder {
	client := dynamodb.New(dynamodb.Options{
		BaseEndpoint: aws.String(endpoint),
		Region:       "us-east-1",
		Credentials:  credentials.NewStaticCredentialsProvider("local", "local", ""),
	})

	return &DynamoDBSeeder{
		client: client,
		logger: logger,
	}
}

func (s *DynamoDBSeeder) PutItem(ctx context.Context, tableName string, itemJSON []byte) error {
	item, err := dynamoconv.UnmarshalDynamoItem(itemJSON)
	if err != nil {
		return fmt.Errorf("parsing item JSON for table %s: %w", tableName, err)
	}

	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("putting item in table %s: %w", tableName, err)
	}

	return nil
}

// toTableInput converts a seed.TableDefinition to a dynamoconv.TableInput.
func toTableInput(td TableDefinition) dynamoconv.TableInput {
	ti := dynamoconv.TableInput{
		Name: td.Name,
		PartitionKey: dynamoconv.KeyField{
			Name:     td.PartitionKey.Name,
			DataType: td.PartitionKey.DataType,
		},
		Fields:        td.Fields,
		StreamEnabled: td.StreamEnabled,
	}

	if td.SortKey != nil {
		ti.SortKey = &dynamoconv.KeyField{
			Name:     td.SortKey.Name,
			DataType: td.SortKey.DataType,
		}
	}

	for _, idx := range td.Indexes {
		di := dynamoconv.IndexInput{Name: idx.Name}
		for _, f := range idx.Fields {
			di.Fields = append(di.Fields, dynamoconv.KeyField{
				Name:     f.Name,
				DataType: f.DataType,
			})
		}
		ti.Indexes = append(ti.Indexes, di)
	}

	return ti
}

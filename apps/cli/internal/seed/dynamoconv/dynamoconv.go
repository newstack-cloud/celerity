// Package dynamoconv provides pure conversion functions between Go types and
// DynamoDB attribute values. These are extracted from the seed package so they
// can be unit-tested without requiring a live DynamoDB connection.
package dynamoconv

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamotypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// TableInput describes a NoSQL table for CreateTable input construction.
// This mirrors seed.TableDefinition but lives here to avoid an import cycle.
type TableInput struct {
	Name          string
	PartitionKey  KeyField
	SortKey       *KeyField
	Indexes       []IndexInput
	Fields        map[string]string
	StreamEnabled bool
}

// KeyField is a name + type pair for a key attribute.
type KeyField struct {
	Name     string
	DataType string
}

// IndexInput describes a secondary index.
type IndexInput struct {
	Name   string
	Fields []KeyField
}

// AttributeTypeMap maps blueprint schema types to DynamoDB scalar attribute types.
var AttributeTypeMap = map[string]dynamotypes.ScalarAttributeType{
	"string":  dynamotypes.ScalarAttributeTypeS,
	"number":  dynamotypes.ScalarAttributeTypeN,
	"binary":  dynamotypes.ScalarAttributeTypeB,
	"boolean": dynamotypes.ScalarAttributeTypeS,
}

// MapAttributeType converts a blueprint schema type to a DynamoDB scalar attribute type.
// Falls back to S (string) for unknown types.
func MapAttributeType(dataType string) dynamotypes.ScalarAttributeType {
	if t, ok := AttributeTypeMap[dataType]; ok {
		return t
	}
	return dynamotypes.ScalarAttributeTypeS
}

// BuildCreateTableInput constructs a DynamoDB CreateTable input from a TableInput.
func BuildCreateTableInput(table TableInput) *dynamodb.CreateTableInput {
	attrDefs := []dynamotypes.AttributeDefinition{
		{
			AttributeName: aws.String(table.PartitionKey.Name),
			AttributeType: MapAttributeType(table.PartitionKey.DataType),
		},
	}

	keySchema := []dynamotypes.KeySchemaElement{
		{
			AttributeName: aws.String(table.PartitionKey.Name),
			KeyType:       dynamotypes.KeyTypeHash,
		},
	}

	if table.SortKey != nil {
		attrDefs = append(attrDefs, dynamotypes.AttributeDefinition{
			AttributeName: aws.String(table.SortKey.Name),
			AttributeType: MapAttributeType(table.SortKey.DataType),
		})
		keySchema = append(keySchema, dynamotypes.KeySchemaElement{
			AttributeName: aws.String(table.SortKey.Name),
			KeyType:       dynamotypes.KeyTypeRange,
		})
	}

	input := &dynamodb.CreateTableInput{
		TableName:            aws.String(table.Name),
		AttributeDefinitions: attrDefs,
		KeySchema:            keySchema,
		BillingMode:          dynamotypes.BillingModePayPerRequest,
	}

	if len(table.Indexes) > 0 {
		input.GlobalSecondaryIndexes = buildGSIs(table)
		input.AttributeDefinitions = appendIndexAttributes(attrDefs, table)
	}

	if table.StreamEnabled {
		input.StreamSpecification = &dynamotypes.StreamSpecification{
			StreamEnabled:  aws.Bool(true),
			StreamViewType: dynamotypes.StreamViewTypeNewAndOldImages,
		}
	}

	return input
}

func buildGSIs(table TableInput) []dynamotypes.GlobalSecondaryIndex {
	gsis := make([]dynamotypes.GlobalSecondaryIndex, 0, len(table.Indexes))
	for _, idx := range table.Indexes {
		if len(idx.Fields) == 0 {
			continue
		}

		gsi := dynamotypes.GlobalSecondaryIndex{
			IndexName: aws.String(idx.Name),
			Projection: &dynamotypes.Projection{
				ProjectionType: dynamotypes.ProjectionTypeAll,
			},
		}

		gsi.KeySchema = []dynamotypes.KeySchemaElement{
			{
				AttributeName: aws.String(idx.Fields[0].Name),
				KeyType:       dynamotypes.KeyTypeHash,
			},
		}
		if len(idx.Fields) > 1 {
			gsi.KeySchema = append(gsi.KeySchema, dynamotypes.KeySchemaElement{
				AttributeName: aws.String(idx.Fields[1].Name),
				KeyType:       dynamotypes.KeyTypeRange,
			})
		}

		gsis = append(gsis, gsi)
	}
	return gsis
}

func appendIndexAttributes(
	existing []dynamotypes.AttributeDefinition,
	table TableInput,
) []dynamotypes.AttributeDefinition {
	defined := map[string]bool{}
	for _, attr := range existing {
		defined[*attr.AttributeName] = true
	}

	for _, idx := range table.Indexes {
		for _, field := range idx.Fields {
			if defined[field.Name] {
				continue
			}
			existing = append(existing, dynamotypes.AttributeDefinition{
				AttributeName: aws.String(field.Name),
				AttributeType: MapAttributeType(field.DataType),
			})
			defined[field.Name] = true
		}
	}

	return existing
}

// UnmarshalDynamoItem converts a plain JSON object into a DynamoDB attribute value map.
// Supports string, number, boolean, null, list, and map types.
func UnmarshalDynamoItem(data []byte) (map[string]dynamotypes.AttributeValue, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	item := make(map[string]dynamotypes.AttributeValue, len(raw))
	for key, val := range raw {
		av, err := MarshalAttributeValue(val)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", key, err)
		}
		item[key] = av
	}

	return item, nil
}

// MarshalAttributeValue converts a Go value to a DynamoDB attribute value.
func MarshalAttributeValue(val any) (dynamotypes.AttributeValue, error) {
	if val == nil {
		return &dynamotypes.AttributeValueMemberNULL{Value: true}, nil
	}

	switch v := val.(type) {
	case string:
		return &dynamotypes.AttributeValueMemberS{Value: v}, nil
	case float64:
		return &dynamotypes.AttributeValueMemberN{Value: fmt.Sprintf("%g", v)}, nil
	case bool:
		return &dynamotypes.AttributeValueMemberBOOL{Value: v}, nil
	case []any:
		return marshalListAttribute(v)
	case map[string]any:
		return marshalMapAttribute(v)
	default:
		return nil, fmt.Errorf("unsupported type %T", val)
	}
}

func marshalListAttribute(items []any) (dynamotypes.AttributeValue, error) {
	list := make([]dynamotypes.AttributeValue, 0, len(items))
	for _, item := range items {
		av, err := MarshalAttributeValue(item)
		if err != nil {
			return nil, err
		}
		list = append(list, av)
	}
	return &dynamotypes.AttributeValueMemberL{Value: list}, nil
}

func marshalMapAttribute(m map[string]any) (dynamotypes.AttributeValue, error) {
	result := make(map[string]dynamotypes.AttributeValue, len(m))
	for k, v := range m {
		av, err := MarshalAttributeValue(v)
		if err != nil {
			return nil, fmt.Errorf("key %s: %w", k, err)
		}
		result[k] = av
	}
	return &dynamotypes.AttributeValueMemberM{Value: result}, nil
}

package dynamoconv

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	dynamotypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/suite"
)

type DynamoConvTestSuite struct {
	suite.Suite
}

func TestDynamoConvTestSuite(t *testing.T) {
	suite.Run(t, new(DynamoConvTestSuite))
}

// --- MapAttributeType ---

func (s *DynamoConvTestSuite) Test_map_attribute_type_string() {
	s.Assert().Equal(dynamotypes.ScalarAttributeTypeS, MapAttributeType("string"))
}

func (s *DynamoConvTestSuite) Test_map_attribute_type_number() {
	s.Assert().Equal(dynamotypes.ScalarAttributeTypeN, MapAttributeType("number"))
}

func (s *DynamoConvTestSuite) Test_map_attribute_type_binary() {
	s.Assert().Equal(dynamotypes.ScalarAttributeTypeB, MapAttributeType("binary"))
}

func (s *DynamoConvTestSuite) Test_map_attribute_type_boolean_maps_to_string() {
	s.Assert().Equal(dynamotypes.ScalarAttributeTypeS, MapAttributeType("boolean"))
}

func (s *DynamoConvTestSuite) Test_map_attribute_type_unknown_defaults_to_string() {
	s.Assert().Equal(dynamotypes.ScalarAttributeTypeS, MapAttributeType("unknown"))
}

// --- BuildCreateTableInput ---

func (s *DynamoConvTestSuite) Test_build_table_partition_key_only() {
	input := BuildCreateTableInput(TableInput{
		Name:         "users",
		PartitionKey: KeyField{Name: "userId", DataType: "string"},
	})

	s.Assert().Equal("users", *input.TableName)
	s.Require().Len(input.KeySchema, 1)
	s.Assert().Equal("userId", *input.KeySchema[0].AttributeName)
	s.Assert().Equal(dynamotypes.KeyTypeHash, input.KeySchema[0].KeyType)
	s.Assert().Nil(input.StreamSpecification)
	s.Assert().Empty(input.GlobalSecondaryIndexes)
}

func (s *DynamoConvTestSuite) Test_build_table_with_sort_key() {
	input := BuildCreateTableInput(TableInput{
		Name:         "orders",
		PartitionKey: KeyField{Name: "customerId", DataType: "string"},
		SortKey:      &KeyField{Name: "orderDate", DataType: "string"},
	})

	s.Require().Len(input.KeySchema, 2)
	s.Assert().Equal("orderDate", *input.KeySchema[1].AttributeName)
	s.Assert().Equal(dynamotypes.KeyTypeRange, input.KeySchema[1].KeyType)
	s.Require().Len(input.AttributeDefinitions, 2)
}

func (s *DynamoConvTestSuite) Test_build_table_with_stream() {
	input := BuildCreateTableInput(TableInput{
		Name:          "events",
		PartitionKey:  KeyField{Name: "id", DataType: "string"},
		StreamEnabled: true,
	})

	s.Require().NotNil(input.StreamSpecification)
	s.Assert().True(*input.StreamSpecification.StreamEnabled)
	s.Assert().Equal(dynamotypes.StreamViewTypeNewAndOldImages, input.StreamSpecification.StreamViewType)
}

func (s *DynamoConvTestSuite) Test_build_table_with_gsi() {
	input := BuildCreateTableInput(TableInput{
		Name:         "users",
		PartitionKey: KeyField{Name: "userId", DataType: "string"},
		Indexes: []IndexInput{
			{
				Name: "email-index",
				Fields: []KeyField{
					{Name: "email", DataType: "string"},
				},
			},
		},
	})

	s.Require().Len(input.GlobalSecondaryIndexes, 1)
	s.Assert().Equal("email-index", *input.GlobalSecondaryIndexes[0].IndexName)

	// email should be added to attribute definitions
	found := false
	for _, attr := range input.AttributeDefinitions {
		if *attr.AttributeName == "email" {
			found = true
		}
	}
	s.Assert().True(found, "expected email in attribute definitions")
}

func (s *DynamoConvTestSuite) Test_build_table_gsi_with_two_fields() {
	input := BuildCreateTableInput(TableInput{
		Name:         "users",
		PartitionKey: KeyField{Name: "userId", DataType: "string"},
		Indexes: []IndexInput{
			{
				Name: "tenant-email-index",
				Fields: []KeyField{
					{Name: "tenantId", DataType: "string"},
					{Name: "email", DataType: "string"},
				},
			},
		},
	})

	gsi := input.GlobalSecondaryIndexes[0]
	s.Require().Len(gsi.KeySchema, 2)
	s.Assert().Equal(dynamotypes.KeyTypeHash, gsi.KeySchema[0].KeyType)
	s.Assert().Equal(dynamotypes.KeyTypeRange, gsi.KeySchema[1].KeyType)
}

func (s *DynamoConvTestSuite) Test_build_table_gsi_empty_fields_skipped() {
	input := BuildCreateTableInput(TableInput{
		Name:         "users",
		PartitionKey: KeyField{Name: "userId", DataType: "string"},
		Indexes:      []IndexInput{{Name: "empty-index", Fields: nil}},
	})

	s.Assert().Empty(input.GlobalSecondaryIndexes)
}

func (s *DynamoConvTestSuite) Test_append_index_attributes_deduplicates() {
	existing := []dynamotypes.AttributeDefinition{
		{AttributeName: aws.String("userId"), AttributeType: dynamotypes.ScalarAttributeTypeS},
	}
	table := TableInput{
		Indexes: []IndexInput{
			{
				Name: "idx",
				Fields: []KeyField{
					{Name: "userId", DataType: "string"}, // already defined
					{Name: "email", DataType: "string"},  // new
				},
			},
		},
	}

	result := appendIndexAttributes(existing, table)
	s.Assert().Len(result, 2) // userId + email, not 3
}

// --- UnmarshalDynamoItem ---

func (s *DynamoConvTestSuite) Test_unmarshal_string_field() {
	item, err := UnmarshalDynamoItem([]byte(`{"name":"Alice"}`))
	s.Require().NoError(err)
	val, ok := item["name"].(*dynamotypes.AttributeValueMemberS)
	s.Require().True(ok)
	s.Assert().Equal("Alice", val.Value)
}

func (s *DynamoConvTestSuite) Test_unmarshal_number_field() {
	item, err := UnmarshalDynamoItem([]byte(`{"age":42}`))
	s.Require().NoError(err)
	val, ok := item["age"].(*dynamotypes.AttributeValueMemberN)
	s.Require().True(ok)
	s.Assert().Equal("42", val.Value)
}

func (s *DynamoConvTestSuite) Test_unmarshal_boolean_field() {
	item, err := UnmarshalDynamoItem([]byte(`{"active":true}`))
	s.Require().NoError(err)
	val, ok := item["active"].(*dynamotypes.AttributeValueMemberBOOL)
	s.Require().True(ok)
	s.Assert().True(val.Value)
}

func (s *DynamoConvTestSuite) Test_unmarshal_null_field() {
	item, err := UnmarshalDynamoItem([]byte(`{"deleted":null}`))
	s.Require().NoError(err)
	_, ok := item["deleted"].(*dynamotypes.AttributeValueMemberNULL)
	s.Assert().True(ok)
}

func (s *DynamoConvTestSuite) Test_unmarshal_list_field() {
	item, err := UnmarshalDynamoItem([]byte(`{"tags":["a","b"]}`))
	s.Require().NoError(err)
	val, ok := item["tags"].(*dynamotypes.AttributeValueMemberL)
	s.Require().True(ok)
	s.Assert().Len(val.Value, 2)
}

func (s *DynamoConvTestSuite) Test_unmarshal_nested_map_field() {
	item, err := UnmarshalDynamoItem([]byte(`{"address":{"city":"London"}}`))
	s.Require().NoError(err)
	val, ok := item["address"].(*dynamotypes.AttributeValueMemberM)
	s.Require().True(ok)
	city, ok := val.Value["city"].(*dynamotypes.AttributeValueMemberS)
	s.Require().True(ok)
	s.Assert().Equal("London", city.Value)
}

func (s *DynamoConvTestSuite) Test_unmarshal_invalid_json_returns_error() {
	_, err := UnmarshalDynamoItem([]byte(`{invalid}`))
	s.Assert().Error(err)
}

// --- MarshalAttributeValue ---

func (s *DynamoConvTestSuite) Test_marshal_nil_returns_null() {
	av, err := MarshalAttributeValue(nil)
	s.Require().NoError(err)
	_, ok := av.(*dynamotypes.AttributeValueMemberNULL)
	s.Assert().True(ok)
}

func (s *DynamoConvTestSuite) Test_marshal_unsupported_type_returns_error() {
	_, err := MarshalAttributeValue(complex(1, 2))
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "unsupported type")
}

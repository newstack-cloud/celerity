package seed

import (
	"context"
	"fmt"

	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"go.uber.org/zap"
)

// DatastoreProvisioner creates tables/buckets from blueprint resource definitions.
type DatastoreProvisioner interface {
	ProvisionTable(ctx context.Context, table TableDefinition) error
}

// BucketProvisioner creates storage buckets.
type BucketProvisioner interface {
	ProvisionBucket(ctx context.Context, bucketName string) error
}

// TableDefinition describes a NoSQL table to provision.
type TableDefinition struct {
	Name          string
	PartitionKey  KeyField
	SortKey       *KeyField
	Indexes       []IndexDefinition
	Fields        map[string]string
	StreamEnabled bool
}

// KeyField is a name + type pair for a key attribute.
type KeyField struct {
	Name     string
	DataType string
}

// IndexDefinition describes a secondary index.
type IndexDefinition struct {
	Name   string
	Fields []KeyField
}

// ProvisionResult tracks what was provisioned for TUI reporting.
type ProvisionResult struct {
	Tables  []string
	Buckets []string
}

// ProvisionFromBlueprint reads the blueprint and provisions tables and buckets
// for local development. streamEnabledTables is a set of datastore resource names
// that should have change streams enabled (e.g. DynamoDB Streams) because a
// consumer is linked to them.
func ProvisionFromBlueprint(
	ctx context.Context,
	bp *schema.Blueprint,
	datastoreProvisioner DatastoreProvisioner,
	bucketProvisioner BucketProvisioner,
	streamEnabledTables map[string]bool,
	logger *zap.Logger,
) (*ProvisionResult, error) {
	if bp.Resources == nil {
		return &ProvisionResult{}, nil
	}

	result := &ProvisionResult{}

	for name, resource := range bp.Resources.Values {
		if resource.Type == nil {
			continue
		}

		switch resource.Type.Value {
		case "celerity/datastore":
			enableStream := streamEnabledTables[name]
			if err := provisionDatastore(ctx, name, resource, datastoreProvisioner, enableStream, logger); err != nil {
				return nil, err
			}
			tableName := extractSpecStringField(resource, "name")
			if tableName == "" {
				tableName = name
			}
			result.Tables = append(result.Tables, tableName)

		case "celerity/bucket":
			bucketName := extractSpecStringField(resource, "name")
			if bucketName == "" {
				bucketName = name
			}
			if err := bucketProvisioner.ProvisionBucket(ctx, bucketName); err != nil {
				return nil, fmt.Errorf("provisioning bucket %s: %w", bucketName, err)
			}
			result.Buckets = append(result.Buckets, bucketName)
		}
	}

	return result, nil
}

func provisionDatastore(
	ctx context.Context,
	resourceName string,
	resource *schema.Resource,
	provisioner DatastoreProvisioner,
	enableStream bool,
	logger *zap.Logger,
) error {
	tableDef, err := extractTableDefinition(resourceName, resource)
	if err != nil {
		return fmt.Errorf("extracting table definition for %s: %w", resourceName, err)
	}
	tableDef.StreamEnabled = enableStream

	logger.Debug("provisioning table",
		zap.String("table", tableDef.Name),
		zap.Bool("streamEnabled", enableStream),
	)
	if err := provisioner.ProvisionTable(ctx, *tableDef); err != nil {
		return fmt.Errorf("provisioning table %s: %w", tableDef.Name, err)
	}

	return nil
}

func extractTableDefinition(
	resourceName string,
	resource *schema.Resource,
) (*TableDefinition, error) {
	spec := resource.Spec
	if spec == nil || spec.Fields == nil {
		return nil, fmt.Errorf("resource %s has no spec fields", resourceName)
	}

	tableName := extractScalarString(spec.Fields["name"])
	if tableName == "" {
		tableName = resourceName
	}

	tableDef := &TableDefinition{
		Name:   tableName,
		Fields: map[string]string{},
	}

	if err := extractKeys(spec, tableDef); err != nil {
		return nil, fmt.Errorf("resource %s: %w", resourceName, err)
	}

	extractFieldTypes(spec, tableDef)
	extractIndexes(spec, tableDef)

	return tableDef, nil
}

func extractKeys(spec *core.MappingNode, tableDef *TableDefinition) error {
	keysNode := spec.Fields["keys"]
	if keysNode == nil || keysNode.Fields == nil {
		return fmt.Errorf("missing required 'keys' in spec")
	}

	pk := extractScalarString(keysNode.Fields["partitionKey"])
	if pk == "" {
		return fmt.Errorf("missing required 'keys.partitionKey' in spec")
	}

	tableDef.PartitionKey = KeyField{
		Name:     pk,
		DataType: lookupFieldType(spec, pk),
	}

	if sk := extractScalarString(keysNode.Fields["sortKey"]); sk != "" {
		tableDef.SortKey = &KeyField{
			Name:     sk,
			DataType: lookupFieldType(spec, sk),
		}
	}

	return nil
}

func extractFieldTypes(spec *core.MappingNode, tableDef *TableDefinition) {
	schemaNode := spec.Fields["schema"]
	if schemaNode == nil || schemaNode.Fields == nil {
		return
	}

	fieldsNode := schemaNode.Fields["fields"]
	if fieldsNode == nil || fieldsNode.Fields == nil {
		return
	}

	for fieldName, fieldNode := range fieldsNode.Fields {
		dataType := extractScalarString(fieldNode)
		if dataType != "" {
			tableDef.Fields[fieldName] = dataType
		}
	}
}

func extractIndexes(spec *core.MappingNode, tableDef *TableDefinition) {
	indexesNode := spec.Fields["indexes"]
	if indexesNode == nil || indexesNode.Items == nil {
		return
	}

	for _, indexNode := range indexesNode.Items {
		if indexNode.Fields == nil {
			continue
		}

		indexName := extractScalarString(indexNode.Fields["name"])
		if indexName == "" {
			continue
		}

		idx := IndexDefinition{Name: indexName}
		fieldsNode := indexNode.Fields["fields"]
		if fieldsNode != nil && fieldsNode.Items != nil {
			extractedFields := extractFields(fieldsNode, spec)
			idx.Fields = append(idx.Fields, extractedFields...)
		}

		tableDef.Indexes = append(tableDef.Indexes, idx)
	}
}

func extractFields(fieldsNode *core.MappingNode, spec *core.MappingNode) []KeyField {
	var fields []KeyField

	for _, fieldItem := range fieldsNode.Items {
		fieldName := extractScalarString(fieldItem)
		if fieldName != "" {
			fields = append(fields, KeyField{
				Name:     fieldName,
				DataType: lookupFieldType(spec, fieldName),
			})
		}
	}

	return fields
}

func lookupFieldType(spec *core.MappingNode, fieldName string) string {
	schemaNode := spec.Fields["schema"]
	if schemaNode == nil || schemaNode.Fields == nil {
		return "string"
	}

	fieldsNode := schemaNode.Fields["fields"]
	if fieldsNode == nil || fieldsNode.Fields == nil {
		return "string"
	}

	fieldType := extractScalarString(fieldsNode.Fields[fieldName])
	if fieldType == "" {
		return "string"
	}
	return fieldType
}

func extractSpecStringField(resource *schema.Resource, field string) string {
	if resource.Spec == nil || resource.Spec.Fields == nil {
		return ""
	}
	return extractScalarString(resource.Spec.Fields[field])
}

func extractScalarString(node *core.MappingNode) string {
	if node == nil || node.Scalar == nil {
		return ""
	}
	if node.Scalar.StringValue != nil {
		return *node.Scalar.StringValue
	}
	return node.Scalar.ToString()
}

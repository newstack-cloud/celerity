package convertv1

import (
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/serialisation"
	"github.com/two-hundred/celerity/libs/plugin-framework/sharedtypesv1"
)

// ToPBResourceDefinitionsSchema converts a provider.ResourceDefinitionsSchema to a
// sharedtypesv1.ResourceDefinitionsSchema that can be sent in a gRPC call to a plugin.
func ToPBResourceDefinitionsSchema(
	schema *provider.ResourceDefinitionsSchema,
) (*sharedtypesv1.ResourceDefinitionsSchema, error) {
	if schema == nil {
		return nil, nil
	}

	attributes, err := toPBSchemaAttributes(schema.Attributes)
	if err != nil {
		return nil, err
	}

	items, err := ToPBResourceDefinitionsSchema(schema.Items)
	if err != nil {
		return nil, err
	}

	mapValues, err := ToPBResourceDefinitionsSchema(schema.MapValues)
	if err != nil {
		return nil, err
	}

	oneOf, err := toPBSchemaList(schema.OneOf)
	if err != nil {
		return nil, err
	}

	schemaDefaultValue, err := serialisation.ToMappingNodePB(
		schema.Default,
		/* optional */ true,
	)
	if err != nil {
		return nil, err
	}

	examples, err := ToPBMappingNodeSlice(
		schema.Examples,
	)
	if err != nil {
		return nil, err
	}

	return &sharedtypesv1.ResourceDefinitionsSchema{
		Type:                 string(schema.Type),
		Label:                schema.Label,
		Description:          schema.Description,
		FormattedDescription: schema.FormattedDescription,
		Attributes:           attributes,
		Items:                items,
		MapValues:            mapValues,
		OneOf:                oneOf,
		Required:             schema.Required,
		Nullable:             schema.Nullable,
		DefaultValue:         schemaDefaultValue,
		Examples:             examples,
		Computed:             schema.Computed,
		MustRecreate:         schema.MustRecreate,
	}, nil
}

func toPBSchemaAttributes(
	attributes map[string]*provider.ResourceDefinitionsSchema,
) (map[string]*sharedtypesv1.ResourceDefinitionsSchema, error) {
	if attributes == nil {
		return nil, nil
	}

	pbAttributes := make(map[string]*sharedtypesv1.ResourceDefinitionsSchema, len(attributes))
	for key, attribute := range attributes {
		pbAttribute, err := ToPBResourceDefinitionsSchema(attribute)
		if err != nil {
			return nil, err
		}

		pbAttributes[key] = pbAttribute
	}

	return pbAttributes, nil
}

func toPBSchemaList(
	schemas []*provider.ResourceDefinitionsSchema,
) ([]*sharedtypesv1.ResourceDefinitionsSchema, error) {
	if schemas == nil {
		return nil, nil
	}

	pbSchemas := make([]*sharedtypesv1.ResourceDefinitionsSchema, len(schemas))
	for i, schema := range schemas {
		pbSchema, err := ToPBResourceDefinitionsSchema(schema)
		if err != nil {
			return nil, err
		}

		pbSchemas[i] = pbSchema
	}

	return pbSchemas, nil
}

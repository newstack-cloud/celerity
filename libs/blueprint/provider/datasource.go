package provider

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
)

// DataSource provides the interface for a data source
// that a provider can contain which provides data that can be used by all
// other resources in the same spec.
type DataSource interface {
	// GetType deals with retrieving the namespaced type for a data source in a blueprint.
	GetType(ctx context.Context, input *DataSourceGetTypeInput) (*DataSourceGetTypeOutput, error)
	// CustomValidate provides support for custom validation that goes beyond
	// the spec schema validation provided by the data source's spec definition.
	CustomValidate(ctx context.Context, input *DataSourceValidateInput) (*DataSourceValidateOutput, error)
	// GetSpecDefinition retrieves the spec definition for a data source.
	// This definition specifies all the fields that can be exported from a data source
	// to be used in a blueprint.
	// This is the first line of validation for a data source in a blueprint and is also
	// useful for validating references to a data source instance
	// in a blueprint and for providing definitions for docs and tooling.
	GetSpecDefinition(ctx context.Context, input *DataSourceGetSpecDefinitionInput) (*DataSourceGetSpecDefinitionOutput, error)
	// GetFilterFields provides the fields that can be used in a filter for a data source.
	GetFilterFields(ctx context.Context, input *DataSourceGetFilterFieldsInput) (*DataSourceGetFilterFieldsOutput, error)
	// Fetch deals with loading the data from the upstream data source
	// and returning the exported fields defined in the spec.
	Fetch(ctx context.Context, input *DataSourceFetchInput) (*DataSourceFetchOutput, error)
}

// DataSourceValidateInput provides the input required to validate
// a data source definition in a blueprint.
type DataSourceValidateInput struct {
	SchemaDataSource *schema.DataSource
	Params           core.BlueprintParams
}

// DataSourceValidateOutput provides the output from validating a data source
// which includes a list of diagnostics that detail issues with the data source.
type DataSourceValidateOutput struct {
	Diagnostics []*core.Diagnostic
}

// DataSourceFetchInput provides the input required to fetch
// data from an upstream data source.
type DataSourceFetchInput struct {
	SchemaDataSource *schema.DataSource
	Params           core.BlueprintParams
}

// DataSourceFetchOutput provides the output from fetching data from an upstream
// data source which includes the exported fields defined in the spec.
type DataSourceFetchOutput struct {
	Data map[string]interface{}
}

// DataSourceGetTypeInput provides the input required to
// retrieve the namespaced type for a data source in a blueprint.
type DataSourceGetTypeInput struct {
	Params core.BlueprintParams
}

// DataSourceGetTypeOutput provides the output from retrieving the namespaced type
// for a data source in a blueprint.
type DataSourceGetTypeOutput struct {
	Type string
}

// DataSourceGetFilterFieldsOutput provides the output from retrieving the fields
// that can be used in a filter for a data source.
type DataSourceGetFilterFieldsInput struct {
	Params core.BlueprintParams
}

// DataSourceGetFilterFieldsOutput provides the output from retrieving the fields
// that can be used in a filter for a data source.
type DataSourceGetFilterFieldsOutput struct {
	Fields []string
}

// DataSourceGetSpecDefinitionInput provides the input data needed for a data source to
// provide a spec definition.
type DataSourceGetSpecDefinitionInput struct {
	Params core.BlueprintParams
}

// DataSourceGetSpecDefinitionOutput provides the output data from providing a spec definition
// for a data source.
type DataSourceGetSpecDefinitionOutput struct {
	SpecDefinition *DataSourceSpecDefinition
}

// DataSourceSpecDefinition provides a definition for a data source spec
// that can be used for validation, docs and tooling.
type DataSourceSpecDefinition struct {
	// Fields holds a mapping of schemas for
	// fields that can be exported from a data source.
	// Unlike resource specs, data source specs are restricted
	// in that they only support primitives or arrays of primitives.
	Fields map[string]*DataSourceSpecSchema
}

// DataSourceSpecSchema provides a schema that can be used to validate
// a data source spec.
type DataSourceSpecSchema struct {
	// Type holds the type of the data source spec.
	Type DataSourceSpecSchemaType
	// Label holds a human-readable label for the data source spec.
	Label string
	// Description holds a human-readable description for the data source spec
	// without any formatting.
	Description string
	// FormattedDescription holds a human-readable description for the data source spec
	// that is formatted with markdown.
	FormattedDescription string
	// Items holds the schema for the items in a data source spec schema array.
	// Items are expected to be of a primitive type, if an array type is provided here,
	// an error will occur.
	Items *DataSourceSpecSchema
	// Nullable specifies whether the data source spec schema can be null.
	// This essentially means that the data source implementation can provide
	// a null value for the field.
	Nullable bool
}

// DataSourceSpecSchemaType holds the type of a data suource schema.
type DataSourceSpecSchemaType string

const (
	// DataSourceSpecTypeString is for a schema string.
	DataSourceSpecTypeString DataSourceSpecSchemaType = "string"
	// DataSourceSpecTypeInteger is for a schema integer.
	DataSourceSpecTypeInteger DataSourceSpecSchemaType = "integer"
	// DataSourceSpecTypeFloat is for a schema float.
	DataSourceSpecTypeFloat DataSourceSpecSchemaType = "float"
	// DataSourceSpecTypeBoolean is for a schema boolean.
	DataSourceSpecTypeBoolean DataSourceSpecSchemaType = "boolean"
	// DataSourceSpecTypeArray is for a schema array.
	DataSourceSpecTypeArray DataSourceSpecSchemaType = "array"
)

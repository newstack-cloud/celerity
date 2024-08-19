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
	// Validate deals with ensuring all the exported fields
	// defined by a user in the spec are supported.
	Validate(ctx context.Context, input *DataSourceValidateInput) (*DataSourceValidateOutput, error)
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

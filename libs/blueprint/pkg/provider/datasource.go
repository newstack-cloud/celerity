package provider

import (
	"context"

	"github.com/freshwebio/celerity/libs/blueprint/pkg/core"
	"github.com/freshwebio/celerity/libs/blueprint/pkg/schema"
)

// DataSource provides the interface for a data source
// that a provider can contain which provides data that can be used by all
// other resources in the same spec.
type DataSource interface {
	// Validate deals with ensuring all the exported fields
	// defined by a user in the spec are supported.
	Validate(ctx context.Context, schemaDataSource *schema.DataSource, params core.BlueprintParams) error
	// Fetch deals with loading the data from the downstream data source
	// and returning the exported fields defined in the spec.
	Fetch(ctx context.Context, schemaDataSource *schema.DataSource, params core.BlueprintParams) (map[string]interface{}, error)
}

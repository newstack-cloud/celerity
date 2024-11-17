package schema

import (
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
)

// ValidateDataSourceSchema is a helper function that allows plugins
// to validate a data source provided in a blueprint against
// a pre-determined schema that declares what fields are avaiable
// to be exported to be used by other items in the same blueprint.
func ValidateDataSourceSchema(
	dataSourceSchema map[string]*Schema,
	blueprintDataSource *schema.DataSource,
	params core.BlueprintParams,
) ([]*core.Diagnostic, error) {
	return nil, nil
}

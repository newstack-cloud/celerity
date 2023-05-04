package validation

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
)

// ValidateDataSource ensures that a given data source matches the specification
// for all cases not handled during schema parsing.
func ValidateDataSource(ctx context.Context, name string, dataSource *schema.DataSource) error {
	if dataSource.Filter == nil {
		return errDataSourceMissingFilter(name)
	}

	if dataSource.Filter.Field == "" {
		return errDataSourceMissingFilterField(name)
	}

	if dataSource.Filter.Search == nil || len(dataSource.Filter.Search.Values) == 0 {
		return errDataSourceMissingFilterSearch(name)
	}

	if dataSource.Exports == nil || len(dataSource.Exports) == 0 {
		return errDataSourceMissingExports(name)
	}

	return nil
}

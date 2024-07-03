package validation

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/source"
)

// ValidateDataSource ensures that a given data source matches the specification
// for all cases not handled during schema parsing.
func ValidateDataSource(
	ctx context.Context,
	name string,
	dataSource *schema.DataSource,
	dataSourceMap *schema.DataSourceMap,
) error {
	if dataSource.Filter == nil {
		return errDataSourceMissingFilter(
			name, getDataSourceMeta(dataSourceMap, name),
		)
	}

	if dataSource.Filter.Field == "" {
		return errDataSourceMissingFilterField(
			name, getDataSourceMeta(dataSourceMap, name),
		)
	}

	if dataSource.Filter.Search == nil || len(dataSource.Filter.Search.Values) == 0 {
		return errDataSourceMissingFilterSearch(
			name, getDataSourceMeta(dataSourceMap, name),
		)
	}

	if dataSource.Exports == nil || len(dataSource.Exports) == 0 {
		return errDataSourceMissingExports(
			name, getDataSourceMeta(dataSourceMap, name),
		)
	}

	return nil
}

func getDataSourceMeta(varMap *schema.DataSourceMap, varName string) *source.Meta {
	if varMap == nil {
		return nil
	}

	return varMap.SourceMeta[varName]
}

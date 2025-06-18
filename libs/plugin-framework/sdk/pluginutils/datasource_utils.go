package pluginutils

import (
	"github.com/newstack-cloud/celerity/libs/blueprint/core"
	"github.com/newstack-cloud/celerity/libs/blueprint/provider"
	"github.com/newstack-cloud/celerity/libs/blueprint/schema"
)

// GetDataSourceFilterOperator is a utility function that safely
// retrieves a data source filter operator from a resolved data source filter
// without producing a nil pointer dereference error.
// An empty string is returned if the operator is not set.
func GetDataSourceFilterOperator(
	dataSourceFilter *provider.ResolvedDataSourceFilter,
) schema.DataSourceFilterOperator {
	if dataSourceFilter == nil || dataSourceFilter.Operator == nil {
		return ""
	}

	return dataSourceFilter.Operator.Value
}

// GetDataSourceFilterSearchValues is a utility function that safely
// retrieves data source filter search values from a resolved data source filter
// without producing a nil pointer dereference error.
// It returns an empty slice if the search values are not set.
func GetDataSourceFilterSearchValues(
	dataSourceFilter *provider.ResolvedDataSourceFilter,
) []*core.MappingNode {
	if dataSourceFilter == nil || dataSourceFilter.Search == nil {
		return []*core.MappingNode{}
	}

	return dataSourceFilter.Search.Values
}

// GetDataSourceFilterSearchValue is a utility function that safely
// retrieves a data source filter search value from a resolved data source filter
// without producing a nil pointer dereference error.
// It returns nil if the search value is not set or if the index is out of bounds.
func GetDataSourceFilterSearchValue(
	dataSourceFilter *provider.ResolvedDataSourceFilter,
	index int,
) *core.MappingNode {
	if dataSourceFilter == nil || dataSourceFilter.Search == nil {
		return nil
	}

	if index < 0 || index >= len(dataSourceFilter.Search.Values) {
		return nil
	}

	return dataSourceFilter.Search.Values[index]
}

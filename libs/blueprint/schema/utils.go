package schema

// GetDataSourceType safely extracts the type from a data source,
// returning an empty string if the type wrapper is nil or empty.
func GetDataSourceType(dataSource *DataSource) string {
	if dataSource.Type == nil || dataSource.Type.Value == "" {
		return ""
	}

	return dataSource.Type.Value
}

// GetResourceType safely extracts the type from a resource,
// returning an empty string if the type wrapper is nil or empty.
func GetResourceType(resource *Resource) string {
	if resource.Type == nil || resource.Type.Value == "" {
		return ""
	}

	return resource.Type.Value
}

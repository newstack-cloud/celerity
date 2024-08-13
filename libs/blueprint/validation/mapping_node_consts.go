package validation

// MappingNodeMaxTraverseDepth is the maximum depth allowed to traverse
// a mapping node tree to search for invalid keys in the pre-validation
// phase of the following blueprints components:
// - Resources[ResourceName].Spec
// - Resources[ResourceName].Metadata.Custom
// - DataSources[DataSourceName].Metadata.Custom
// - Include.Variables
// - Include.Metadata
// - Metadata
const MappingNodeMaxTraverseDepth = 10

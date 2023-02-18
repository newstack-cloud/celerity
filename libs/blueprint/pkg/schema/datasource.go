package schema

import (
	"encoding/json"
	"fmt"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/common/pkg/core"
	"gopkg.in/yaml.v3"
)

// DataSource represents a blueprint
// data source in the specification.
// Data sources are accessible to all resources in a blueprint via the
// ${dataSources.{dataSourceName}.{exportedField}} reference.
// For example, you would access a data source called network with an exported
// vpc field via ${dataSources.network.vpc}.
type DataSource struct {
	Type               string                            `yaml:"type" json:"type"`
	DataSourceMetadata *Metadata                         `yaml:"metadata" json:"metadata"`
	Exports            map[string]*DataSourceFieldExport `yaml:"exports" json:"exports"`
}

// Variable provides the definition of an exported field
// from a data source in a blueprint.
type DataSourceFieldExport struct {
	Type        *DataSourceFieldTypeWrapper `yaml:"type" json:"type"`
	Description string                      `yaml:"description,omitempty" json:"description,omitempty"`
}

// DataSourceMetadata represents the metadata associated
// with a blueprint data source that can be used to provide
// annotations that are used to configure data sources when fetching data
// from the data source provider.
type DataSourceMetadata struct {
	DisplayName string                        `yaml:"displayName" json:"displayName"`
	Annotations map[string]bpcore.ScalarValue `yaml:"annotations" json:"annotations"`
}

// DataSourceFieldTypeWrapper provides a struct that holds a data source field type
// value.
// The reason that this exists is to allow more fine-grained control
// when serialising and deserialising data source field exports in a blueprint
// so we can check precise values.
type DataSourceFieldTypeWrapper struct {
	Value DataSourceFieldType
}

func (t *DataSourceFieldTypeWrapper) MarshalYAML() (interface{}, error) {
	if !core.SliceContains(DataSourceFieldTypes, t.Value) {
		return nil, errInvalidDataSourceFieldType(t.Value)
	}

	return t.Value, nil
}

func (t *DataSourceFieldTypeWrapper) UnmarshalYAML(value *yaml.Node) error {
	valueDataSourceFieldType := DataSourceFieldType(value.Value)
	if !core.SliceContains(DataSourceFieldTypes, valueDataSourceFieldType) {
		return errInvalidDataSourceFieldType(valueDataSourceFieldType)
	}

	t.Value = valueDataSourceFieldType
	return nil
}

func (t *DataSourceFieldTypeWrapper) MarshalJSON() ([]byte, error) {
	if !core.SliceContains(DataSourceFieldTypes, t.Value) {
		return nil, errInvalidDataSourceFieldType(t.Value)
	}
	return []byte(fmt.Sprintf("\"%s\"", t.Value)), nil
}

func (t *DataSourceFieldTypeWrapper) UnmarshalJSON(data []byte) error {
	var typeVal string
	err := json.Unmarshal(data, &typeVal)
	if err != nil {
		return err
	}

	typeValDataSourceFieldType := DataSourceFieldType(typeVal)
	if !core.SliceContains(DataSourceFieldTypes, typeValDataSourceFieldType) {
		return errInvalidDataSourceFieldType(typeValDataSourceFieldType)
	}
	t.Value = DataSourceFieldType(typeVal)

	return nil
}

// DataSourceFieldType represents a type of exported field
// for a data source defined in a blueprint.
// Can be one of "string", "integer", "float", "boolean", "array" or "object".
type DataSourceFieldType string

func (t DataSourceFieldType) Equal(compareWith DataSourceFieldType) bool {
	return t == compareWith
}

const (
	// DataSourceFieldTypeString is for an exported
	// string field from a data source in a blueprint.
	DataSourceFieldTypeString DataSourceFieldType = "string"
	// DataSourceFieldTypeInteger is for an exported
	// integer field from a data source in a blueprint.
	DataSourceFieldTypeInteger DataSourceFieldType = "integer"
	// DataSourceFieldTypeFloat is for an exported
	// float field from a data source in a blueprint.
	DataSourceFieldTypeFloat DataSourceFieldType = "float"
	// DataSourceFieldTypeBoolean is for an exported
	// boolean field from a data source in a blueprint.
	DataSourceFieldTypeBoolean DataSourceFieldType = "boolean"
	// DataSourceFieldTypeArray is for an exported
	// array field from a data source in a blueprint.
	DataSourceFieldTypeArray DataSourceFieldType = "array"
	// DataSourceFieldTypeObject is for an exported
	// array field from a data source in a blueprint.
	DataSourceFieldTypeObject DataSourceFieldType = "object"
)

var (
	// DataSourceFieldTypes provides a slice of all the supported
	// data source field types to be used for clean validation of fields
	// with a field with DataSourceFieldType.
	DataSourceFieldTypes = []DataSourceFieldType{
		DataSourceFieldTypeString,
		DataSourceFieldTypeInteger,
		DataSourceFieldTypeFloat,
		DataSourceFieldTypeBoolean,
		DataSourceFieldTypeArray,
		DataSourceFieldTypeObject,
	}
)

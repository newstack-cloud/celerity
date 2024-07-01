package schema

import (
	"encoding/json"
	"fmt"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/substitutions"
	"github.com/two-hundred/celerity/libs/common/pkg/core"
	"gopkg.in/yaml.v3"
)

// DataSource represents a blueprint
// data source in the specification.
// Data sources are accessible to all resources in a blueprint via the
// ${datasources.{dataSourceName}.{exportedField}} reference.
// For example, you would access a data source called network with an exported
// vpc field via ${datasources.network.vpc}.
type DataSource struct {
	Type               string                               `yaml:"type" json:"type"`
	DataSourceMetadata *DataSourceMetadata                  `yaml:"metadata" json:"metadata"`
	Filter             *DataSourceFilter                    `yaml:"filter" json:"filter"`
	Exports            map[string]*DataSourceFieldExport    `yaml:"exports" json:"exports"`
	Description        *substitutions.StringOrSubstitutions `yaml:"description,omitempty" json:"description,omitempty"`
}

// DataSourceFilter provides the definition of a filter
// used to select a specific data source instance from a provider.
type DataSourceFilter struct {
	Field    string                           `yaml:"field" json:"field"`
	Operator *DataSourceFilterOperatorWrapper `yaml:"operator" json:"operator"`
	Search   *DataSourceFilterSearch          `yaml:"search" json:"search"`
}

// DataSourceFilterSearch provides the definition of one or more
// search values for a data source filter.
type DataSourceFilterSearch struct {
	Values []*substitutions.StringOrSubstitutions
}

func (s *DataSourceFilterSearch) MarshalYAML() (interface{}, error) {
	// Export as a single value if there is only one value,
	// in a similar way to how the user would define a single search value
	// for a filter.
	if len(s.Values) == 1 {
		return s.Values[0], nil
	}

	return s.Values, nil
}

func (s *DataSourceFilterSearch) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.SequenceNode {
		values := []*substitutions.StringOrSubstitutions{}
		for _, node := range value.Content {
			value := &substitutions.StringOrSubstitutions{}
			err := value.UnmarshalYAML(node)
			if err != nil {
				return err
			}
			values = append(values, value)
		}
		s.Values = values
	}

	singleValue := &substitutions.StringOrSubstitutions{}
	err := singleValue.UnmarshalYAML(value)
	if err != nil {
		return err
	}

	s.Values = []*substitutions.StringOrSubstitutions{singleValue}
	return nil
}

func (s *DataSourceFilterSearch) MarshalJSON() ([]byte, error) {
	if len(s.Values) == 1 {
		return json.Marshal(s.Values[0])
	}

	return json.Marshal(s.Values)
}

func (s *DataSourceFilterSearch) UnmarshalJSON(data []byte) error {
	singleValue := substitutions.StringOrSubstitutions{}
	err := json.Unmarshal(data, &singleValue)
	if err == nil {
		s.Values = []*substitutions.StringOrSubstitutions{&singleValue}
		return nil
	}

	multipleValues := []*substitutions.StringOrSubstitutions{}
	err = json.Unmarshal(data, &multipleValues)
	if err != nil {
		return err
	}

	s.Values = multipleValues
	return nil
}

// DataSourceFilterOperatorWrapper provides a struct that holds a data source filter operator
// value.
// The reason that this exists is to allow more fine-grained control
// when serialising and deserialising data source filter operators in a blueprint
// so we can check precise values.
type DataSourceFilterOperatorWrapper struct {
	Value DataSourceFilterOperator
}

func (w *DataSourceFilterOperatorWrapper) MarshalYAML() (interface{}, error) {
	if !core.SliceContains(DataSourceFilterOperators, w.Value) {
		return nil, errInvalidDataSourceFilterOperator(w.Value)
	}

	return w.Value, nil
}

func (w *DataSourceFilterOperatorWrapper) UnmarshalYAML(value *yaml.Node) error {
	valueFilterOperator := DataSourceFilterOperator(value.Value)
	if !core.SliceContains(DataSourceFilterOperators, valueFilterOperator) {
		return errInvalidDataSourceFilterOperator(valueFilterOperator)
	}

	w.Value = valueFilterOperator
	return nil
}

func (w *DataSourceFilterOperatorWrapper) MarshalJSON() ([]byte, error) {
	if !core.SliceContains(DataSourceFilterOperators, w.Value) {
		return nil, errInvalidDataSourceFilterOperator(w.Value)
	}
	return []byte(fmt.Sprintf("\"%s\"", w.Value)), nil
}

func (w *DataSourceFilterOperatorWrapper) UnmarshalJSON(data []byte) error {
	var typeVal string
	err := json.Unmarshal(data, &typeVal)
	if err != nil {
		return err
	}

	typeValDataSourceFilterOperator := DataSourceFilterOperator(typeVal)
	if !core.SliceContains(DataSourceFilterOperators, typeValDataSourceFilterOperator) {
		return errInvalidDataSourceFilterOperator(typeValDataSourceFilterOperator)
	}
	w.Value = typeValDataSourceFilterOperator

	return nil
}

// DataSourceFilterOperator represents a filter operator
// for a data source defined in a blueprint.
// Can be one of "=", "!=", "in", "not in", "has key", "not has key",
// "contains", "not contains", "starts with", "not starts with", "ends with" or
// "not ends with".
type DataSourceFilterOperator string

func (t DataSourceFilterOperator) Equal(compareWith DataSourceFilterOperator) bool {
	return t == compareWith
}

const (
	// DataSourceFilterOperatorEquals represents the "=" filter operator.
	DataSourceFilterOperatorEquals DataSourceFilterOperator = "="
	// DataSourceFilterOperatorNotEquals represents the "!=" filter operator.
	DataSourceFilterOperatorNotEquals DataSourceFilterOperator = "!="
	// DataSourceFilterOperatorIn represents the "in" filter operator.
	DataSourceFilterOperatorIn DataSourceFilterOperator = "in"
	// DataSourceFilterOperatorNotIn represents the "not in" filter operator.
	DataSourceFilterOperatorNotIn DataSourceFilterOperator = "not in"
	// DataSourceFilterOperatorHasKey represents the "has key" filter operator.
	DataSourceFilterOperatorHasKey DataSourceFilterOperator = "has key"
	// DataSourceFilterOperatorNotHasKey represents the "not has key" filter operator.
	DataSourceFilterOperatorNotHasKey DataSourceFilterOperator = "not has key"
	// DataSourceFilterOperatorContains represents the "contains" filter operator.
	DataSourceFilterOperatorContains DataSourceFilterOperator = "contains"
	// DataSourceFilterOperatorNotContains represents the "not contains" filter operator.
	DataSourceFilterOperatorNotContains DataSourceFilterOperator = "not contains"
	// DataSourceFilterOperatorStartsWith represents the "starts with" filter operator.
	DataSourceFilterOperatorStartsWith DataSourceFilterOperator = "starts with"
	// DataSourceFilterOperatorNotStartsWith represents the "not starts with" filter operator.
	DataSourceFilterOperatorNotStartsWith DataSourceFilterOperator = "not starts with"
	// DataSourceFilterOperatorEndsWith represents the "ends with" filter operator.
	DataSourceFilterOperatorEndsWith DataSourceFilterOperator = "ends with"
	// DataSourceFilterOperatorNotEndsWith represents the "not ends with" filter operator.
	DataSourceFilterOperatorNotEndsWith DataSourceFilterOperator = "not ends with"
)

var (
	// DataSourceFilterOperators provides a slice of all the supported
	// data source filter operators.
	DataSourceFilterOperators = []DataSourceFilterOperator{
		DataSourceFilterOperatorEquals,
		DataSourceFilterOperatorNotEquals,
		DataSourceFilterOperatorIn,
		DataSourceFilterOperatorNotIn,
		DataSourceFilterOperatorHasKey,
		DataSourceFilterOperatorNotHasKey,
		DataSourceFilterOperatorContains,
		DataSourceFilterOperatorNotContains,
		DataSourceFilterOperatorStartsWith,
		DataSourceFilterOperatorNotStartsWith,
		DataSourceFilterOperatorEndsWith,
		DataSourceFilterOperatorNotEndsWith,
	}
)

// DataSourceFieldExport provides the definition of an exported field
// from a data source in a blueprint.
type DataSourceFieldExport struct {
	Type        *DataSourceFieldTypeWrapper          `yaml:"type" json:"type"`
	AliasFor    string                               `yaml:"aliasFor" json:"aliasFor"`
	Description *substitutions.StringOrSubstitutions `yaml:"description,omitempty" json:"description,omitempty"`
}

// DataSourceMetadata represents the metadata associated
// with a blueprint data source that can be used to provide
// annotations that are used to configure data sources when fetching data
// from the data source provider.
type DataSourceMetadata struct {
	DisplayName *substitutions.StringOrSubstitutions            `yaml:"displayName" json:"displayName"`
	Annotations map[string]*substitutions.StringOrSubstitutions `yaml:"annotations,omitempty" json:"annotations,omitempty"`
	Custom      *bpcore.MappingNode                             `yaml:"custom,omitempty" json:"custom,omitempty"`
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
		return nil, errInvalidDataSourceFieldType(t.Value, nil, nil)
	}

	return t.Value, nil
}

func (t *DataSourceFieldTypeWrapper) UnmarshalYAML(value *yaml.Node) error {
	valueDataSourceFieldType := DataSourceFieldType(value.Value)
	if !core.SliceContains(DataSourceFieldTypes, valueDataSourceFieldType) {
		return errInvalidDataSourceFieldType(
			valueDataSourceFieldType,
			&value.Line,
			&value.Column,
		)
	}

	t.Value = valueDataSourceFieldType
	return nil
}

func (t *DataSourceFieldTypeWrapper) MarshalJSON() ([]byte, error) {
	if !core.SliceContains(DataSourceFieldTypes, t.Value) {
		return nil, errInvalidDataSourceFieldType(t.Value, nil, nil)
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
		return errInvalidDataSourceFieldType(typeValDataSourceFieldType, nil, nil)
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

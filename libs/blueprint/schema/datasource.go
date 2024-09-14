package schema

import (
	"encoding/json"
	"fmt"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/jsonutils"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"gopkg.in/yaml.v3"
)

// DataSource represents a blueprint
// data source in the specification.
// Data sources are accessible to all resources in a blueprint via the
// ${datasources.{dataSourceName}.{exportedField}} reference.
// For example, you would access a data source called network with an exported
// vpc field via ${datasources.network.vpc}.
type DataSource struct {
	Type               *DataSourceTypeWrapper               `yaml:"type" json:"type"`
	DataSourceMetadata *DataSourceMetadata                  `yaml:"metadata" json:"metadata"`
	Filter             *DataSourceFilter                    `yaml:"filter" json:"filter"`
	Exports            *DataSourceFieldExportMap            `yaml:"exports" json:"exports"`
	Description        *substitutions.StringOrSubstitutions `yaml:"description,omitempty" json:"description,omitempty"`
	SourceMeta         *source.Meta                         `yaml:"-" json:"-"`
}

func (s *DataSource) UnmarshalYAML(value *yaml.Node) error {
	s.SourceMeta = &source.Meta{
		Line:   value.Line,
		Column: value.Column,
	}

	type dataSourceAlias DataSource
	var alias dataSourceAlias
	if err := value.Decode(&alias); err != nil {
		return wrapErrorWithLineInfo(err, value)
	}

	s.Type = alias.Type
	s.DataSourceMetadata = alias.DataSourceMetadata
	s.Filter = alias.Filter
	s.Exports = alias.Exports
	s.Description = alias.Description

	return nil
}

// DataSourceTypeWrapper provides a struct that holds a data source type
// value.
type DataSourceTypeWrapper struct {
	Value      string
	SourceMeta *source.Meta
}

func (t *DataSourceTypeWrapper) MarshalYAML() (interface{}, error) {
	return t.Value, nil
}

func (t *DataSourceTypeWrapper) UnmarshalYAML(value *yaml.Node) error {
	t.SourceMeta = &source.Meta{
		Line:   value.Line,
		Column: value.Column,
	}

	t.Value = value.Value
	return nil
}

func (t *DataSourceTypeWrapper) MarshalJSON() ([]byte, error) {
	escaped := jsonutils.EscapeJSONString(string(t.Value))
	return []byte(fmt.Sprintf("\"%s\"", escaped)), nil
}

func (t *DataSourceTypeWrapper) UnmarshalJSON(data []byte) error {
	var typeVal string
	err := json.Unmarshal(data, &typeVal)
	if err != nil {
		return err
	}

	t.Value = typeVal

	return nil
}

// DataSourceFieldExportMap provides a mapping of names to
// data source field exports.
// This includes extra information about the locations of
// the keys in the original source being unmarshalled.
// This information will not always be present, it is populated
// when unmarshalling from YAML source documents.
type DataSourceFieldExportMap struct {
	Values map[string]*DataSourceFieldExport
	// Mapping of exported field names to their source locations.
	SourceMeta map[string]*source.Meta
}

func (m *DataSourceFieldExportMap) MarshalYAML() (interface{}, error) {
	return m.Values, nil
}

func (m *DataSourceFieldExportMap) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return errInvalidMap(value, "exports")
	}

	m.Values = make(map[string]*DataSourceFieldExport)
	m.SourceMeta = make(map[string]*source.Meta)
	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i]
		val := value.Content[i+1]

		m.SourceMeta[key.Value] = &source.Meta{
			Line:   key.Line,
			Column: key.Column,
		}

		var export DataSourceFieldExport
		err := val.Decode(&export)
		if err != nil {
			return err
		}

		m.Values[key.Value] = &export
	}

	return nil
}

func (m *DataSourceFieldExportMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Values)
}

func (m *DataSourceFieldExportMap) UnmarshalJSON(data []byte) error {
	values := make(map[string]*DataSourceFieldExport)
	err := json.Unmarshal(data, &values)
	if err != nil {
		return err
	}

	m.Values = values
	return nil
}

// DataSourceFilter provides the definition of a filter
// used to select a specific data source instance from a provider.
type DataSourceFilter struct {
	Field      *bpcore.ScalarValue              `yaml:"field" json:"field"`
	Operator   *DataSourceFilterOperatorWrapper `yaml:"operator" json:"operator"`
	Search     *DataSourceFilterSearch          `yaml:"search" json:"search"`
	SourceMeta *source.Meta                     `yaml:"-" json:"-"`
}

func (f *DataSourceFilter) UnmarshalYAML(value *yaml.Node) error {
	f.SourceMeta = &source.Meta{
		Line:   value.Line,
		Column: value.Column,
	}

	type dataSourceFilterAlias DataSourceFilter
	var alias dataSourceFilterAlias
	if err := value.Decode(&alias); err != nil {
		return wrapErrorWithLineInfo(err, value)
	}

	f.Field = alias.Field
	f.Operator = alias.Operator
	f.Search = alias.Search

	return nil
}

// DataSourceFilterSearch provides the definition of one or more
// search values for a data source filter.
type DataSourceFilterSearch struct {
	Values     []*substitutions.StringOrSubstitutions
	SourceMeta *source.Meta
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
	s.SourceMeta = &source.Meta{
		Line:   value.Line,
		Column: value.Column,
	}

	if value.Kind == yaml.SequenceNode {
		values := []*substitutions.StringOrSubstitutions{}
		for _, node := range value.Content {
			value := &substitutions.StringOrSubstitutions{}
			err := value.UnmarshalYAML(node)
			if err != nil {
				return wrapErrorWithLineInfo(err, node)
			}
			values = append(values, value)
		}
		s.Values = values
	}

	singleValue := &substitutions.StringOrSubstitutions{}
	err := singleValue.UnmarshalYAML(value)
	if err != nil {
		return wrapErrorWithLineInfo(err, value)
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
	Value      DataSourceFilterOperator
	SourceMeta *source.Meta
}

func (w *DataSourceFilterOperatorWrapper) MarshalYAML() (interface{}, error) {
	return w.Value, nil
}

func (w *DataSourceFilterOperatorWrapper) UnmarshalYAML(value *yaml.Node) error {
	w.SourceMeta = &source.Meta{
		Line:   value.Line,
		Column: value.Column,
	}
	valueFilterOperator := DataSourceFilterOperator(value.Value)

	w.Value = valueFilterOperator
	return nil
}

func (w *DataSourceFilterOperatorWrapper) MarshalJSON() ([]byte, error) {
	escaped := jsonutils.EscapeJSONString(string(w.Value))
	return []byte(fmt.Sprintf("\"%s\"", escaped)), nil
}

func (w *DataSourceFilterOperatorWrapper) UnmarshalJSON(data []byte) error {
	var typeVal string
	err := json.Unmarshal(data, &typeVal)
	if err != nil {
		return err
	}

	typeValDataSourceFilterOperator := DataSourceFilterOperator(typeVal)
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
	AliasFor    *bpcore.ScalarValue                  `yaml:"aliasFor" json:"aliasFor"`
	Description *substitutions.StringOrSubstitutions `yaml:"description,omitempty" json:"description,omitempty"`
	SourceMeta  *source.Meta                         `yaml:"-" json:"-"`
}

func (e *DataSourceFieldExport) UnmarshalYAML(value *yaml.Node) error {
	e.SourceMeta = &source.Meta{
		Line:   value.Line,
		Column: value.Column,
	}

	type dataSourceFieldExportAlias DataSourceFieldExport
	var alias dataSourceFieldExportAlias
	if err := value.Decode(&alias); err != nil {
		return wrapErrorWithLineInfo(err, value)
	}

	e.Type = alias.Type
	e.AliasFor = alias.AliasFor
	e.Description = alias.Description

	return nil
}

// DataSourceMetadata represents the metadata associated
// with a blueprint data source that can be used to provide
// annotations that are used to configure data sources when fetching data
// from the data source provider.
type DataSourceMetadata struct {
	DisplayName *substitutions.StringOrSubstitutions `yaml:"displayName" json:"displayName"`
	Annotations *StringOrSubstitutionsMap            `yaml:"annotations,omitempty" json:"annotations,omitempty"`
	Custom      *bpcore.MappingNode                  `yaml:"custom,omitempty" json:"custom,omitempty"`
	SourceMeta  *source.Meta                         `yaml:"-" json:"-"`
}

func (m *DataSourceMetadata) UnmarshalYAML(value *yaml.Node) error {
	m.SourceMeta = &source.Meta{
		Line:   value.Line,
		Column: value.Column,
	}

	type dataSourceMetadataAlias DataSourceMetadata
	var alias dataSourceMetadataAlias
	if err := value.Decode(&alias); err != nil {
		return wrapErrorWithLineInfo(err, value)
	}

	m.DisplayName = alias.DisplayName
	m.Annotations = alias.Annotations
	m.Custom = alias.Custom

	return nil
}

// DataSourceFieldTypeWrapper provides a struct that holds a data source field type
// value.
// The reason that this exists is to allow more fine-grained control
// when serialising and deserialising data source field exports in a blueprint
// so we can check precise values.
type DataSourceFieldTypeWrapper struct {
	Value      DataSourceFieldType
	SourceMeta *source.Meta
}

func (t *DataSourceFieldTypeWrapper) MarshalYAML() (interface{}, error) {
	return t.Value, nil
}

func (t *DataSourceFieldTypeWrapper) UnmarshalYAML(value *yaml.Node) error {
	t.SourceMeta = &source.Meta{
		Line:   value.Line,
		Column: value.Column,
	}
	valueDataSourceFieldType := DataSourceFieldType(value.Value)

	t.Value = valueDataSourceFieldType
	return nil
}

func (t *DataSourceFieldTypeWrapper) MarshalJSON() ([]byte, error) {
	escaped := jsonutils.EscapeJSONString(string(t.Value))
	return []byte(fmt.Sprintf("\"%s\"", escaped)), nil
}

func (t *DataSourceFieldTypeWrapper) UnmarshalJSON(data []byte) error {
	var typeVal string
	err := json.Unmarshal(data, &typeVal)
	if err != nil {
		return err
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
	}
)

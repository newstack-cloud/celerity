package substitutions

// StringOrSubstitution represents a value that can be either
// a string or a substitution represented as ${..}
// from user input.
type StringOrSubstitution struct {
	StringValue       *string
	SubstitutionValue *Substitution
}

// Substitution is a representation of a placeholder provided
// with the ${..} syntax.
type Substitution struct {
	Function           *SubstitutionFunction
	Variable           *SubstitutionVariable
	DataSourceProperty *SubstitutionDataSourceProperty
	ResourceProperty   *SubstitutionResourceProperty
	Child              *SubstitutionChild
	StringValue        *string
	IntValue           *int64
	FloatValue         *float64
	BoolValue          *bool
}

type SubstitutionVariable struct {
	VariableName string
}

type SubstitutionDataSourceProperty struct {
	DataSourceName    string
	FieldName         string
	PrimitiveArrIndex *int64
}

type SubstitutionResourceProperty struct {
	ResourceName string
	Path         []*SubstitutionPathItem
}

type SubstitutionPathItem struct {
	FieldName         string
	PrimitiveArrIndex *int64
}

type SubstitutionChild struct {
	ChildName string
	Path      []*SubstitutionPathItem
}

type SubstitutionFunction struct {
	FunctionName SubstitutionFunctionName
	Arguments    []*Substitution
}

// SubstitutionFunctionName is a type alias reserved for names
// of functions that can be called in a substitution.
type SubstitutionFunctionName string

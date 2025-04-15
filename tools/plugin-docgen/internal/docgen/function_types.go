package docgen

type FunctionDefinition struct {
	Parameters []*FunctionParameter `json:"parameters"`
	Return     *FunctionReturn      `json:"return"`
}

type FunctionParameter struct {
	ParamType      string `json:"paramType"`
	Name           string `json:"name,omitempty"`
	Label          string `json:"label,omitempty"`
	Description    string `json:"description,omitempty"`
	AllowNullValue bool   `json:"allowNullValue"`
	Optional       bool   `json:"optional"`

	// This should only be present for scalar, object, function and variadic parameters.
	ValueTypeDefinition *ValueTypeDefinition `json:"valueTypeDefinition,omitempty"`

	// This should only be present for list parameters.
	ElementValueTypeDefinition *ValueTypeDefinition `json:"elementValueTypeDefinition,omitempty"`

	// This should only be present for map parameters.
	MapValueTypeDefinition *ValueTypeDefinition `json:"mapValueTypeDefinition,omitempty"`

	// This should only be present for "any" parameters that are union types.
	UnionValueTypeDefinitions []*ValueTypeDefinition `json:"unionValueTypeDefinitions,omitempty"`

	// The following fields should only present for variadic parameters.
	VariadicSingleType bool `json:"singleType,omitempty"`
	VariadicNamed      bool `json:"named,omitempty"`
}

type FunctionReturn struct {
	ReturnType  string `json:"returnType"`
	Description string `json:"description,omitempty"`

	// This should only be present for scalar, object, function return types.
	ValueTypeDefinition *ValueTypeDefinition `json:"valueTypeDefinition,omitempty"`

	// This should only be present for list return types.
	ElementValueTypeDefinition *ValueTypeDefinition `json:"elementValueTypeDefinition,omitempty"`

	// This should only be present for map return types.
	MapValueTypeDefinition *ValueTypeDefinition `json:"mapValueTypeDefinition,omitempty"`

	// This should only be present for "any" return types that are union types.
	UnionValueTypeDefinitions []*ValueTypeDefinition `json:"unionValueTypeDefinitions,omitempty"`
}

type ValueTypeDefinition struct {
	Type        string `json:"type"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`

	// This should only be present for "string" scalar types.
	StringChoices []string `json:"stringChoices,omitempty"`

	// This should only be present for list types.
	ElementValueTypeDefinition *ValueTypeDefinition `json:"elementValueTypeDefinition,omitempty"`

	// This should only be present for map types.
	MapValueTypeDefinition *ValueTypeDefinition `json:"mapValueTypeDefinition,omitempty"`

	// This should only be present for object types.
	AttributeValueTypeDefinitions map[string]*AttributeType `json:"attributeValueTypeDefinitions,omitempty"`

	// This should only be present for function types.
	FunctionDefinition *FunctionDefinition `json:"functionDefinition,omitempty"`

	// This should only be present for "any" types that are union types.
	UnionValueTypeDefinitions []*ValueTypeDefinition `json:"unionValueTypeDefinitions,omitempty"`
}

type AttributeType struct {
	ValueTypeDefinition
	Nullable bool `json:"nullable"`
}

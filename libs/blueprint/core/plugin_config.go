package core

// ConfigDefinition contains a detailed definition (schema) of the configuration
// required for a provider or transformer plugin.
// Fields that have dynamic keys should use the `<placeholder>` syntax
// in the key name, e.g. "aws.config.regionKMSKeys.<region>".
// Schema validation will match based on the pattern for dynamic keys.
// Dynamic keys are often useful to emulate nested dynamic map or array structures
// in provider and transformer configuration.
type ConfigDefinition struct {
	Fields                map[string]*ConfigFieldDefinition `json:"fields"`
	AllowAdditionalFields bool                              `json:"allowAdditionalFields"`
}

// ConfigFieldDefinition represents a field in a configuration definition
// for a provider or transformer plugin.
type ConfigFieldDefinition struct {
	Type        ScalarType `json:"type"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	// Set to true if the config field should be a secret,
	// An application such as the deploy engine
	// will ensure that the value of this field
	// does not appear in logs.
	Secret        bool           `json:"secret"`
	DefaultValue  *ScalarValue   `json:"defaultValue,omitempty"`
	AllowedValues []*ScalarValue `json:"allowedValues,omitempty"`
	Examples      []*ScalarValue `json:"examples,omitempty"`
	Required      bool           `json:"required"`
}

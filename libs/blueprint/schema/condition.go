package schema

import (
	"encoding/json"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"gopkg.in/yaml.v3"
)

// Condition represents a condition that can be used
// to determine if a resource should be created.
type Condition struct {
	// A list of conditions that must all be true.
	And []*Condition `yaml:"and,omitempty" json:"and,omitempty"`
	// A list of conditions where at least one must be true.
	Or []*Condition `yaml:"or,omitempty" json:"or,omitempty"`
	// A condition that will be negated.
	Not *Condition `yaml:"not,omitempty" json:"not,omitempty"`
	// A condition expression that is expected
	// to be a substitution that resolves to a boolean.
	StringValue *substitutions.StringOrSubstitutions `yaml:"-" json:"-"`
	SourceMeta  *source.Meta                         `yaml:"-" json:"-"`
}

func (c *Condition) UnmarshalYAML(value *yaml.Node) error {
	c.SourceMeta = &source.Meta{
		Position: source.Position{
			Line:   value.Line,
			Column: value.Column,
		},
	}

	if value.Kind == yaml.ScalarNode {
		subOrStringVal := &substitutions.StringOrSubstitutions{}
		if err := value.Decode(subOrStringVal); err == nil {
			c.StringValue = subOrStringVal
			return nil
		} else {
			return err
		}
	}

	type conditionAlias Condition
	var alias conditionAlias
	if err := value.Decode(&alias); err != nil {
		return wrapErrorWithLineInfo(err, value)
	}

	c.And = alias.And
	c.Or = alias.Or
	c.Not = alias.Not

	if (len(c.And) > 0 && len(c.Or) > 0) ||
		(len(c.Or) > 0 && c.Not != nil) ||
		(len(c.And) > 0 && c.Not != nil) {
		return errInvalidResourceCondition(value)
	}

	return nil
}

func (c *Condition) MarshalYAML() (interface{}, error) {
	if c.StringValue != nil {
		return c.StringValue, nil
	}

	type conditionAlias Condition
	var alias conditionAlias
	alias.And = c.And
	alias.Or = c.Or
	alias.Not = c.Not
	return alias, nil
}

func (c *Condition) UnmarshalJSON(data []byte) error {
	if strings.HasPrefix(string(data), "\"") {
		stringVal := &substitutions.StringOrSubstitutions{}
		if err := json.Unmarshal(data, &stringVal); err == nil {
			c.StringValue = stringVal
			return nil
		} else {
			return err
		}
	}

	type conditionAlias Condition
	var alias conditionAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	c.And = alias.And
	c.Or = alias.Or
	c.Not = alias.Not

	if (len(c.And) > 0 && len(c.Or) > 0) ||
		(len(c.Or) > 0 && c.Not != nil) ||
		(len(c.And) > 0 && c.Not != nil) {
		return errInvalidResourceCondition(nil)
	}

	return nil
}

func (c *Condition) MarshalJSON() ([]byte, error) {
	if c.StringValue != nil {
		return json.Marshal(c.StringValue)
	}

	type conditionAlias Condition
	var alias conditionAlias
	alias.And = c.And
	alias.Or = c.Or
	alias.Not = c.Not
	return json.Marshal(alias)
}

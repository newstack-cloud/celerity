package schema

// Blueprint provides the type for a blueprint
// specification loaded into memory.
type Blueprint struct {
	Version     string                 `yaml:"version" json:"version"`
	Transform   *TransformValueWrapper `yaml:"transform" json:"transform"`
	Variables   map[string]*Variable   `yaml:"variables" json:"variables"`
	Resources   map[string]*Resource   `yaml:"resources" json:"resources"`
	DataSources map[string]*DataSource `yaml:"dataSources" json:"dataSources"`
	Exports     map[string]*Export     `yaml:"exports" json:"exports"`
}

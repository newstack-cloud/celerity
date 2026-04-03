package devstubs

// ServiceConfig is the service-level configuration parsed from
// stubs/<service>/service.yaml.
type ServiceConfig struct {
	Port            int               `yaml:"port"`
	ConfigKey       string            `yaml:"configKey"`
	ConfigNamespace string            `yaml:"configNamespace,omitempty"`
	DefaultResponse map[string]any    `yaml:"defaultResponse,omitempty"`
}

// EndpointStubFile is the structure of a single endpoint stub file
// (e.g., stubs/<service>/charges.yaml).
type EndpointStubFile struct {
	Endpoint EndpointDef `yaml:"endpoint"`
	Stubs    []StubDef   `yaml:"stubs"`
}

// EndpointDef identifies the HTTP endpoint this stub file covers.
type EndpointDef struct {
	Method string `yaml:"method"`
	Path   string `yaml:"path"`
}

// StubDef is a single stub scenario for an endpoint.
// Predicates and Responses use mountebank's native format.
type StubDef struct {
	Name       string `yaml:"name"`
	Predicates []any  `yaml:"predicates,omitempty"`
	Responses  []any  `yaml:"responses"`
}

// LoadedService is the fully loaded and parsed stub configuration for
// a single service directory.
type LoadedService struct {
	Name      string
	Config    ServiceConfig
	Endpoints []EndpointStubFile
}

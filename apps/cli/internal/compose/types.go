package compose

// ServiceMapping defines how a blueprint resource type maps to a local Docker Compose service.
type ServiceMapping struct {
	ResourceType   string
	ServiceName    string
	Image          string
	Ports          []PortMapping
	Environment    map[string]string
	Command        []string
	HealthCheck    *HealthCheck
	RuntimeEnvVars map[string]string
}

// PortMapping maps a host port to a container port.
type PortMapping struct {
	Host      string
	Container string
}

// HealthCheck defines a Docker Compose health check.
type HealthCheck struct {
	Test        []string
	Interval    string
	Timeout     string
	Retries     int
	StartPeriod string
}

// ComposeConfig represents the generated Docker Compose configuration.
type ComposeConfig struct {
	Services            map[string]*ComposeService
	RuntimeEnvVars      map[string]string
	// HostEnvVars contains the same endpoints as RuntimeEnvVars but rewritten
	// for host access (localhost with host-mapped ports). Use these when
	// connecting from the host machine (seeding, test runners) rather than
	// from within the Docker network.
	HostEnvVars         map[string]string
	ProjectName         string
	FilePath            string
	StreamEnabledTables map[string]bool
	// StubServices holds loaded stub service configs for the orchestrator
	// to seed config values and load mountebank imposters after compose up.
	StubServices []StubServiceInfo
}

// StubServiceInfo carries the information needed to seed config values
// and load mountebank imposters for a stub service.
type StubServiceInfo struct {
	Name            string
	Port            int // Container port (used for Docker-internal URLs)
	HostPort        int // Host-mapped port (container port + offset)
	ConfigKey       string
	ConfigNamespace string
}

// ComposeService represents a single service in the compose file.
type ComposeService struct {
	Image       string                       `yaml:"image"`
	Ports       []string                     `yaml:"ports,omitempty"`
	Environment map[string]string            `yaml:"environment,omitempty"`
	Command     []string                     `yaml:"command,omitempty"`
	Volumes     []string                     `yaml:"volumes,omitempty"`
	HealthCheck *ComposeHealth               `yaml:"healthcheck,omitempty"`
	DependsOn   map[string]ServiceDependency `yaml:"depends_on,omitempty"`
}

// ServiceDependency describes a service dependency with a condition.
type ServiceDependency struct {
	Condition string `yaml:"condition"`
}

// ComposeHealth is the YAML-serialisable health check for compose files.
type ComposeHealth struct {
	Test        []string `yaml:"test"`
	Interval    string   `yaml:"interval"`
	Timeout     string   `yaml:"timeout"`
	Retries     int      `yaml:"retries"`
	StartPeriod string   `yaml:"start_period"`
}

// composeFile is the top-level Docker Compose file structure for YAML serialisation.
type composeFile struct {
	Services map[string]*ComposeService `yaml:"services"`
}

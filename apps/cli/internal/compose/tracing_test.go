package compose

import (
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// A session's traces are read in a collector it runs itself, so what has to be
// right is that the application is told where to export and the developer is
// told where to look.
type TracingServiceSuite struct {
	suite.Suite
	logger *zap.Logger
}

func TestTracingServiceSuite(t *testing.T) { suite.Run(t, new(TracingServiceSuite)) }

func (s *TracingServiceSuite) SetupTest() {
	s.logger = zap.NewNop()
}

// An API alone, so nothing in the blueprint could imply a collector: it is
// there because it was asked for.
const tracingBlueprint = `
version: 2025-11-02
resources:
  ordersApi:
    type: "celerity/api"
    spec:
      name: orders
`

func (s *TracingServiceSuite) generate(portOffset int, tracing bool) *ComposeConfig {
	s.T().Helper()
	bp, err := schema.LoadString(tracingBlueprint, schema.YAMLSpecFormat)
	s.Require().NoError(err)

	cfg, err := GenerateComposeConfig(
		bp, DeployTargetAWS, "test-project", s.T().TempDir(), portOffset, true, tracing, s.logger,
	)
	s.Require().NoError(err)
	return cfg
}

func (s *TracingServiceSuite) Test_no_collector_runs_unless_one_is_asked_for() {
	cfg := s.generate(0, false)

	s.Assert().NotContains(cfg.Services, ServiceNameTracing)
	s.Assert().NotContains(cfg.RuntimeEnvVars, EnvTelemetryEnabled)
	s.Assert().Empty(cfg.TracingUIURL)
}

func (s *TracingServiceSuite) Test_the_collector_serves_its_ui_and_ingests_otlp() {
	cfg := s.generate(0, true)

	service, running := cfg.Services[ServiceNameTracing]
	s.Require().True(running)
	s.Assert().Equal(tracingImage, service.Image)
	s.Assert().ElementsMatch([]string{"16686:16686", "4317:4317"}, service.Ports)
	s.Require().NotNil(service.HealthCheck)
	s.Assert().Contains(service.HealthCheck.Test, "wget",
		"the image carries no curl")
}

// Enabling is not enough on its own: an SDK builds no exporter until it is
// told tracing is on, so an endpoint without the switch exports nothing.
func (s *TracingServiceSuite) Test_the_application_is_told_tracing_is_on_and_where_to_export() {
	cfg := s.generate(0, true)

	s.Assert().Equal("true", cfg.RuntimeEnvVars[EnvTelemetryEnabled])
	s.Assert().Equal(
		"http://"+ServiceNameTracing+":4317",
		cfg.RuntimeEnvVars[EnvTraceEndpoint],
		"the compose network name, since the application container is on it",
	)
}

// A suite run by `celerity dev test` runs on the host, so a handler it
// exercises in process cannot reach a compose service by name.
func (s *TracingServiceSuite) Test_the_host_endpoint_reaches_the_published_port() {
	cfg := s.generate(0, true)

	s.Assert().Equal("http://localhost:4317", cfg.HostEnvVars[EnvTraceEndpoint])
}

// dev run and dev test both bring a stack up, so the second has to land
// somewhere the first is not already listening.
func (s *TracingServiceSuite) Test_a_second_stack_offsets_both_ports() {
	cfg := s.generate(100, true)

	service := cfg.Services[ServiceNameTracing]
	s.Assert().ElementsMatch([]string{"16786:16686", "4417:4317"}, service.Ports)
	s.Assert().Equal("http://localhost:16786", cfg.TracingUIURL)
	s.Assert().Equal("http://localhost:4417", cfg.HostEnvVars[EnvTraceEndpoint])
}

func (s *TracingServiceSuite) Test_the_ui_url_is_reported_for_the_session_to_print() {
	s.Assert().Equal("http://localhost:16686", s.generate(0, true).TracingUIURL)
}

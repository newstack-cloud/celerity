package devstubs

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ValidatorTestSuite struct {
	suite.Suite
}

func TestValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(ValidatorTestSuite))
}

var okResponse = []any{map[string]any{"is": map[string]any{"statusCode": 200}}}
var createdResponse = []any{map[string]any{"is": map[string]any{"statusCode": 201}}}

func (s *ValidatorTestSuite) Test_valid_service_returns_no_errors() {
	services := []LoadedService{
		{
			Name:   "payments",
			Config: ServiceConfig{Port: 8080, ConfigKey: "payments_url"},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "GET", Path: "/charges"},
					Stubs:    []StubDef{{Name: "ok", Responses: okResponse}},
				},
			},
		},
	}
	errs := ValidateStubs(services)
	s.Assert().Empty(errs)
}

func (s *ValidatorTestSuite) Test_zero_port_is_rejected() {
	services := []LoadedService{
		{
			Name:   "svc",
			Config: ServiceConfig{Port: 0, ConfigKey: "key"},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "GET", Path: "/"},
					Stubs:    []StubDef{{Name: "ok", Responses: okResponse}},
				},
			},
		},
	}
	errs := ValidateStubs(services)
	s.Require().Len(errs, 1)
	s.Assert().Contains(errs[0].Message, "port is required")
}

func (s *ValidatorTestSuite) Test_duplicate_ports_across_services() {
	services := []LoadedService{
		{
			Name:   "svc-a",
			Config: ServiceConfig{Port: 9000, ConfigKey: "key_a"},
			Endpoints: []EndpointStubFile{
				{Endpoint: EndpointDef{Method: "GET", Path: "/"}, Stubs: []StubDef{{Responses: okResponse}}},
			},
		},
		{
			Name:   "svc-b",
			Config: ServiceConfig{Port: 9000, ConfigKey: "key_b"},
			Endpoints: []EndpointStubFile{
				{Endpoint: EndpointDef{Method: "GET", Path: "/"}, Stubs: []StubDef{{Responses: okResponse}}},
			},
		},
	}
	errs := ValidateStubs(services)
	s.Require().Len(errs, 1)
	s.Assert().Contains(errs[0].Message, "conflicts with service")
	s.Assert().Contains(errs[0].Message, "9000")
}

func (s *ValidatorTestSuite) Test_missing_config_key() {
	services := []LoadedService{
		{
			Name:   "svc",
			Config: ServiceConfig{Port: 8080, ConfigKey: ""},
			Endpoints: []EndpointStubFile{
				{Endpoint: EndpointDef{Method: "GET", Path: "/"}, Stubs: []StubDef{{Responses: okResponse}}},
			},
		},
	}
	errs := ValidateStubs(services)
	s.Require().Len(errs, 1)
	s.Assert().Contains(errs[0].Message, "configKey is required")
}

func (s *ValidatorTestSuite) Test_duplicate_config_keys() {
	services := []LoadedService{
		{
			Name:   "svc-a",
			Config: ServiceConfig{Port: 8001, ConfigKey: "same_key"},
			Endpoints: []EndpointStubFile{
				{Endpoint: EndpointDef{Method: "GET", Path: "/"}, Stubs: []StubDef{{Responses: okResponse}}},
			},
		},
		{
			Name:   "svc-b",
			Config: ServiceConfig{Port: 8002, ConfigKey: "same_key"},
			Endpoints: []EndpointStubFile{
				{Endpoint: EndpointDef{Method: "GET", Path: "/"}, Stubs: []StubDef{{Responses: okResponse}}},
			},
		},
	}
	errs := ValidateStubs(services)
	s.Require().Len(errs, 1)
	s.Assert().Contains(errs[0].Message, "configKey")
	s.Assert().Contains(errs[0].Message, "conflicts")
}

func (s *ValidatorTestSuite) Test_no_endpoints_is_rejected() {
	services := []LoadedService{
		{
			Name:      "svc",
			Config:    ServiceConfig{Port: 8080, ConfigKey: "key"},
			Endpoints: nil,
		},
	}
	errs := ValidateStubs(services)
	s.Require().Len(errs, 1)
	s.Assert().Contains(errs[0].Message, "no endpoint stub files found")
}

func (s *ValidatorTestSuite) Test_missing_method_on_endpoint() {
	services := []LoadedService{
		{
			Name:   "svc",
			Config: ServiceConfig{Port: 8080, ConfigKey: "key"},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "", Path: "/users"},
					Stubs:    []StubDef{{Responses: okResponse}},
				},
			},
		},
	}
	errs := ValidateStubs(services)
	s.Assert().NotEmpty(errs)
	found := false
	for _, e := range errs {
		if e.Message == "endpoint.method is required" {
			found = true
		}
	}
	s.Assert().True(found, "expected method required error")
}

func (s *ValidatorTestSuite) Test_missing_path_on_endpoint() {
	services := []LoadedService{
		{
			Name:   "svc",
			Config: ServiceConfig{Port: 8080, ConfigKey: "key"},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "POST", Path: ""},
					Stubs:    []StubDef{{Responses: createdResponse}},
				},
			},
		},
	}
	errs := ValidateStubs(services)
	s.Assert().NotEmpty(errs)
	found := false
	for _, e := range errs {
		if e.Message == "endpoint.path is required" {
			found = true
		}
	}
	s.Assert().True(found, "expected path required error")
}

func (s *ValidatorTestSuite) Test_no_stubs_on_endpoint() {
	services := []LoadedService{
		{
			Name:   "svc",
			Config: ServiceConfig{Port: 8080, ConfigKey: "key"},
			Endpoints: []EndpointStubFile{
				{Endpoint: EndpointDef{Method: "GET", Path: "/users"}, Stubs: nil},
			},
		},
	}
	errs := ValidateStubs(services)
	s.Require().Len(errs, 1)
	s.Assert().Contains(errs[0].Message, "at least one stub is required")
}

func (s *ValidatorTestSuite) Test_missing_responses_on_stub() {
	services := []LoadedService{
		{
			Name:   "svc",
			Config: ServiceConfig{Port: 8080, ConfigKey: "key"},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "GET", Path: "/users"},
					Stubs:    []StubDef{{Name: "bad", Responses: nil}},
				},
			},
		},
	}
	errs := ValidateStubs(services)
	s.Require().Len(errs, 1)
	s.Assert().Contains(errs[0].Message, "at least one response is required")
}

func (s *ValidatorTestSuite) Test_validation_error_formatting_with_all_fields() {
	e := ValidationError{Service: "payments", File: "GET /charges", StubName: "not-found", Message: "bad"}
	s.Assert().Equal("[payments/GET /charges/not-found] bad", e.Error())
}

func (s *ValidatorTestSuite) Test_validation_error_formatting_without_optional_fields() {
	e := ValidationError{Service: "payments", Message: "no endpoints"}
	s.Assert().Equal("[payments] no endpoints", e.Error())
}

func (s *ValidatorTestSuite) Test_multiple_errors_accumulated() {
	services := []LoadedService{
		{
			Name:   "svc",
			Config: ServiceConfig{Port: 0, ConfigKey: ""},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "", Path: ""},
					Stubs:    []StubDef{{Responses: nil}},
				},
			},
		},
	}
	errs := ValidateStubs(services)
	// port=0, configKey="", method="", path="", responses=nil
	s.Assert().GreaterOrEqual(len(errs), 4)
}

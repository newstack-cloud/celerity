package devstubs

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type MountebankTestSuite struct {
	suite.Suite
}

func TestMountebankTestSuite(t *testing.T) {
	suite.Run(t, new(MountebankTestSuite))
}

func (s *MountebankTestSuite) Test_to_imposters_basic_service() {
	services := []LoadedService{
		{
			Name:   "payments",
			Config: ServiceConfig{Port: 9001},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "GET", Path: "/charges"},
					Stubs: []StubDef{
						{
							Name:      "ok",
							Responses: []any{map[string]any{"is": map[string]any{"statusCode": 200, "body": "[]"}}},
						},
					},
				},
			},
		},
	}

	imposters := ToImposters(services)
	s.Require().Len(imposters, 1)
	s.Assert().Equal(9001, imposters[0].Port)
	s.Assert().Equal("http", imposters[0].Protocol)
	s.Assert().Equal("payments", imposters[0].Name)
	s.Require().Len(imposters[0].Stubs, 1)
	s.Assert().Len(imposters[0].Stubs[0].Predicates, 1)
	s.Assert().Len(imposters[0].Stubs[0].Responses, 1)
}

func (s *MountebankTestSuite) Test_to_imposters_predicated_stubs_before_defaults() {
	services := []LoadedService{
		{
			Name:   "svc",
			Config: ServiceConfig{Port: 8080},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "GET", Path: "/users"},
					Stubs: []StubDef{
						{
							Name:      "default",
							Responses: []any{map[string]any{"is": map[string]any{"statusCode": 200}}},
						},
						{
							Name:       "admin",
							Predicates: []any{map[string]any{"equals": map[string]any{"headers": map[string]any{"X-Role": "admin"}}}},
							Responses:  []any{map[string]any{"is": map[string]any{"statusCode": 200}}},
						},
					},
				},
			},
		},
	}

	imposters := ToImposters(services)
	stubs := imposters[0].Stubs
	s.Require().Len(stubs, 2)
	// Predicated stub should come first.
	s.Assert().Len(stubs[0].Predicates, 2) // endpoint equals + header predicate
	s.Assert().Len(stubs[1].Predicates, 1) // just endpoint equals (default)
}

func (s *MountebankTestSuite) Test_to_imposters_with_query_predicate() {
	services := []LoadedService{
		{
			Name:   "svc",
			Config: ServiceConfig{Port: 8080},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "GET", Path: "/search"},
					Stubs: []StubDef{
						{
							Predicates: []any{map[string]any{"equals": map[string]any{"query": map[string]any{"q": "test"}}}},
							Responses:  []any{map[string]any{"is": map[string]any{"statusCode": 200}}},
						},
					},
				},
			},
		},
	}

	imposters := ToImposters(services)
	stubs := imposters[0].Stubs
	s.Require().Len(stubs, 1)
	s.Assert().Len(stubs[0].Predicates, 2) // endpoint equals + query predicate
}

func (s *MountebankTestSuite) Test_to_imposters_with_body_contains_predicate() {
	services := []LoadedService{
		{
			Name:   "svc",
			Config: ServiceConfig{Port: 8080},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "POST", Path: "/orders"},
					Stubs: []StubDef{
						{
							Predicates: []any{map[string]any{"contains": map[string]any{"body": map[string]any{"status": "pending"}}}},
							Responses:  []any{map[string]any{"is": map[string]any{"statusCode": 201}}},
						},
					},
				},
			},
		},
	}

	imposters := ToImposters(services)
	stubs := imposters[0].Stubs
	s.Assert().Len(stubs[0].Predicates, 2) // endpoint equals + body contains
}

func (s *MountebankTestSuite) Test_to_imposters_default_response_from_service_config() {
	services := []LoadedService{
		{
			Name: "svc",
			Config: ServiceConfig{
				Port: 8080,
				DefaultResponse: map[string]any{
					"headers": map[string]any{"Content-Type": "application/json"},
				},
			},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "GET", Path: "/"},
					Stubs: []StubDef{
						{
							Responses: []any{map[string]any{"is": map[string]any{"statusCode": 200}}},
						},
					},
				},
			},
		},
	}

	imposters := ToImposters(services)
	s.Assert().Equal(
		map[string]any{"headers": map[string]any{"Content-Type": "application/json"}},
		imposters[0].DefaultResponse,
	)
}

func (s *MountebankTestSuite) Test_to_imposters_proxy_response() {
	services := []LoadedService{
		{
			Name:   "svc",
			Config: ServiceConfig{Port: 8080},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "GET", Path: "/"},
					Stubs: []StubDef{
						{
							Responses: []any{map[string]any{"proxy": map[string]any{
								"to":   "https://upstream.example.com",
								"mode": "proxyAlways",
							}}},
						},
					},
				},
			},
		},
	}

	imposters := ToImposters(services)
	s.Require().Len(imposters[0].Stubs, 1)
	s.Assert().Len(imposters[0].Stubs[0].Responses, 1)
}

func (s *MountebankTestSuite) Test_to_imposters_multiple_services() {
	services := []LoadedService{
		{
			Name:   "a",
			Config: ServiceConfig{Port: 9001},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "GET", Path: "/"},
					Stubs:    []StubDef{{Responses: []any{map[string]any{"is": map[string]any{"statusCode": 200}}}}},
				},
			},
		},
		{
			Name:   "b",
			Config: ServiceConfig{Port: 9002},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "POST", Path: "/"},
					Stubs:    []StubDef{{Responses: []any{map[string]any{"is": map[string]any{"statusCode": 201}}}}},
				},
			},
		},
	}

	imposters := ToImposters(services)
	s.Assert().Len(imposters, 2)
	s.Assert().Equal(9001, imposters[0].Port)
	s.Assert().Equal(9002, imposters[1].Port)
}

func (s *MountebankTestSuite) Test_to_imposters_no_default_response_when_not_set() {
	services := []LoadedService{
		{
			Name:   "svc",
			Config: ServiceConfig{Port: 8080},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "GET", Path: "/"},
					Stubs:    []StubDef{{Responses: []any{map[string]any{"is": map[string]any{"statusCode": 200}}}}},
				},
			},
		},
	}

	imposters := ToImposters(services)
	s.Assert().Nil(imposters[0].DefaultResponse)
}

func (s *MountebankTestSuite) Test_to_imposters_skips_auto_predicate_when_method_in_predicates() {
	services := []LoadedService{
		{
			Name:   "svc",
			Config: ServiceConfig{Port: 8080},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "GET", Path: "/users"},
					Stubs: []StubDef{
						{
							Predicates: []any{map[string]any{"matches": map[string]any{"method": "GET", "path": "/users/.*"}}},
							Responses:  []any{map[string]any{"is": map[string]any{"statusCode": 200}}},
						},
					},
				},
			},
		},
	}

	imposters := ToImposters(services)
	stubs := imposters[0].Stubs
	s.Require().Len(stubs, 1)
	// Only the user-defined predicate, no auto-injected equals.
	s.Assert().Len(stubs[0].Predicates, 1)
	pred := stubs[0].Predicates[0].(map[string]any)
	s.Assert().Contains(pred, "matches")
}

func (s *MountebankTestSuite) Test_to_imposters_skips_auto_predicate_when_path_in_predicates() {
	services := []LoadedService{
		{
			Name:   "svc",
			Config: ServiceConfig{Port: 8080},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "POST", Path: "/orders"},
					Stubs: []StubDef{
						{
							Predicates: []any{map[string]any{"deepEquals": map[string]any{"path": "/orders"}}},
							Responses:  []any{map[string]any{"is": map[string]any{"statusCode": 201}}},
						},
					},
				},
			},
		},
	}

	imposters := ToImposters(services)
	stubs := imposters[0].Stubs
	s.Require().Len(stubs, 1)
	s.Assert().Len(stubs[0].Predicates, 1)
}

func (s *MountebankTestSuite) Test_to_imposters_auto_predicate_added_when_no_method_or_path() {
	services := []LoadedService{
		{
			Name:   "svc",
			Config: ServiceConfig{Port: 8080},
			Endpoints: []EndpointStubFile{
				{
					Endpoint: EndpointDef{Method: "GET", Path: "/search"},
					Stubs: []StubDef{
						{
							Predicates: []any{map[string]any{"equals": map[string]any{"headers": map[string]any{"Accept": "application/json"}}}},
							Responses:  []any{map[string]any{"is": map[string]any{"statusCode": 200}}},
						},
					},
				},
			},
		},
	}

	imposters := ToImposters(services)
	stubs := imposters[0].Stubs
	s.Require().Len(stubs, 1)
	// Auto-injected equals + user header predicate.
	s.Assert().Len(stubs[0].Predicates, 2)
}

func (s *MountebankTestSuite) Test_to_imposters_empty_services() {
	imposters := ToImposters(nil)
	s.Assert().Empty(imposters)
}

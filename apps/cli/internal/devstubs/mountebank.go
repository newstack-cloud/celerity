package devstubs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type MountebankImposter struct {
	Port            int              `json:"port"`
	Protocol        string           `json:"protocol"`
	Name            string           `json:"name,omitempty"`
	DefaultResponse map[string]any   `json:"defaultResponse,omitempty"`
	Stubs           []MountebankStub `json:"stubs"`
}

type MountebankStub struct {
	Predicates []any `json:"predicates"`
	Responses  []any `json:"responses"`
}

// ToImposters converts loaded services into mountebank imposter payloads.
// Each service becomes one imposter on its configured port. The endpoint
// method and path are prepended as an equals predicate to each stub.
func ToImposters(services []LoadedService) []MountebankImposter {
	var imposters []MountebankImposter

	for _, svc := range services {
		imposter := MountebankImposter{
			Port:            svc.Config.Port,
			Protocol:        "http",
			Name:            svc.Name,
			DefaultResponse: svc.Config.DefaultResponse,
		}

		for _, ep := range svc.Endpoints {
			var withPredicates, defaults []MountebankStub
			for _, stub := range ep.Stubs {
				mbStub := toMountebankStub(ep.Endpoint, stub)
				if len(stub.Predicates) > 0 {
					withPredicates = append(withPredicates, mbStub)
				} else {
					defaults = append(defaults, mbStub)
				}
			}
			imposter.Stubs = append(imposter.Stubs, withPredicates...)
			imposter.Stubs = append(imposter.Stubs, defaults...)
		}

		imposters = append(imposters, imposter)
	}

	return imposters
}

func toMountebankStub(endpoint EndpointDef, stub StubDef) MountebankStub {
	if predicatesContainRequestField(stub.Predicates) {
		return MountebankStub{
			Predicates: stub.Predicates,
			Responses:  stub.Responses,
		}
	}

	predicates := []any{
		map[string]any{
			"equals": map[string]any{
				"method": endpoint.Method,
				"path":   endpoint.Path,
			},
		},
	}
	predicates = append(predicates, stub.Predicates...)

	return MountebankStub{
		Predicates: predicates,
		Responses:  stub.Responses,
	}
}

func predicatesContainRequestField(predicates []any) bool {
	for _, p := range predicates {
		m, ok := p.(map[string]any)
		if !ok {
			continue
		}
		for _, v := range m {
			inner, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if _, hasMethod := inner["method"]; hasMethod {
				return true
			}
			if _, hasPath := inner["path"]; hasPath {
				return true
			}
		}
	}
	return false
}

// LoadImposters creates imposters in a running mountebank instance via its
// REST API.
func LoadImposters(ctx context.Context, mountebankURL string, imposters []MountebankImposter) error {
	client := &http.Client{Timeout: 10 * time.Second}

	for _, imp := range imposters {
		body, err := json.Marshal(imp)
		if err != nil {
			return fmt.Errorf("marshalling imposter %q: %w", imp.Name, err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, mountebankURL+"/imposters", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("creating request for imposter %q: %w", imp.Name, err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("posting imposter %q: %w", imp.Name, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("mountebank returned %d for imposter %q (expected 201)", resp.StatusCode, imp.Name)
		}
	}

	return nil
}

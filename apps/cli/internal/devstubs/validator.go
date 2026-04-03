package devstubs

import (
	"fmt"
	"strings"
)

// ValidationError describes a single validation issue.
type ValidationError struct {
	Service  string
	File     string
	StubName string
	Message  string
}

func (e ValidationError) Error() string {
	parts := []string{e.Service}
	if e.File != "" {
		parts = append(parts, e.File)
	}
	if e.StubName != "" {
		parts = append(parts, e.StubName)
	}
	return fmt.Sprintf("[%s] %s", strings.Join(parts, "/"), e.Message)
}

// ValidateStubs checks all loaded services for configuration errors.
func ValidateStubs(services []LoadedService) []ValidationError {
	var errs []ValidationError

	ports := map[int]string{}
	configKeys := map[string]string{}

	for _, svc := range services {
		errs = append(errs, validateService(svc, ports, configKeys)...)
	}

	return errs
}

func validateService(svc LoadedService, ports map[int]string, configKeys map[string]string) []ValidationError {
	var errs []ValidationError

	if svc.Config.Port == 0 {
		errs = append(errs, ValidationError{
			Service: svc.Name,
			File:    "service.yaml",
			Message: "port is required and must be > 0",
		})
	} else if other, ok := ports[svc.Config.Port]; ok {
		errs = append(errs, ValidationError{
			Service: svc.Name,
			File:    "service.yaml",
			Message: fmt.Sprintf("port %d conflicts with service %q", svc.Config.Port, other),
		})
	} else {
		ports[svc.Config.Port] = svc.Name
	}

	if svc.Config.ConfigKey == "" {
		errs = append(errs, ValidationError{
			Service: svc.Name,
			File:    "service.yaml",
			Message: "configKey is required",
		})
	} else if other, ok := configKeys[svc.Config.ConfigKey]; ok {
		errs = append(errs, ValidationError{
			Service: svc.Name,
			File:    "service.yaml",
			Message: fmt.Sprintf("configKey %q conflicts with service %q", svc.Config.ConfigKey, other),
		})
	} else {
		configKeys[svc.Config.ConfigKey] = svc.Name
	}

	if len(svc.Endpoints) == 0 {
		errs = append(errs, ValidationError{
			Service: svc.Name,
			Message: "no endpoint stub files found",
		})
	}

	for _, ep := range svc.Endpoints {
		errs = append(errs, validateEndpoint(svc.Name, ep)...)
	}

	return errs
}

func validateEndpoint(serviceName string, ep EndpointStubFile) []ValidationError {
	var errs []ValidationError
	file := ep.Endpoint.Method + " " + ep.Endpoint.Path

	if ep.Endpoint.Method == "" {
		errs = append(errs, ValidationError{
			Service: serviceName,
			File:    file,
			Message: "endpoint.method is required",
		})
	}

	if ep.Endpoint.Path == "" {
		errs = append(errs, ValidationError{
			Service: serviceName,
			File:    file,
			Message: "endpoint.path is required",
		})
	}

	if len(ep.Stubs) == 0 {
		errs = append(errs, ValidationError{
			Service: serviceName,
			File:    file,
			Message: "at least one stub is required",
		})
	}

	for _, stub := range ep.Stubs {
		if len(stub.Responses) == 0 {
			errs = append(errs, ValidationError{
				Service:  serviceName,
				File:     file,
				StubName: stub.Name,
				Message:  "at least one response is required",
			})
		}
	}

	return errs
}

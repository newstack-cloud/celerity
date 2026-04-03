package devstubs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadStubs reads all service directories under <appDir>/stubs/ and returns
// the loaded service configurations with their endpoint stubs.
// Returns nil with no error if the stubs directory does not exist.
func LoadStubs(appDir string) ([]LoadedService, error) {
	stubsDir := filepath.Join(appDir, "stubs")
	entries, err := os.ReadDir(stubsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading stubs directory: %w", err)
	}

	var services []LoadedService
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		svc, err := loadService(stubsDir, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("loading stub service %q: %w", entry.Name(), err)
		}
		services = append(services, *svc)
	}

	return services, nil
}

func loadService(stubsDir, serviceName string) (*LoadedService, error) {
	serviceDir := filepath.Join(stubsDir, serviceName)

	configPath := filepath.Join(serviceDir, "service.yaml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading service.yaml: %w", err)
	}

	var config ServiceConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("parsing service.yaml: %w", err)
	}

	entries, err := os.ReadDir(serviceDir)
	if err != nil {
		return nil, fmt.Errorf("reading service directory: %w", err)
	}

	var endpoints []EndpointStubFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "service.yaml" {
			continue
		}
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		endpointPath := filepath.Join(serviceDir, name)
		data, err := os.ReadFile(endpointPath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}

		var endpoint EndpointStubFile
		if err := yaml.Unmarshal(data, &endpoint); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", name, err)
		}
		endpoints = append(endpoints, endpoint)
	}

	return &LoadedService{
		Name:      serviceName,
		Config:    config,
		Endpoints: endpoints,
	}, nil
}

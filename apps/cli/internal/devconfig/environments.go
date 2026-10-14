package devconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// EnvironmentInfo describes one deployed environment the CLI can target.
//
// An environment is a deploy config file rather than anything the CLI tracks
// of its own: app.deploy.jsonc is the default one, and app.deploy.<name>.jsonc
// adds another. Nothing has to be registered, so adding an environment is
// adding a file.
type EnvironmentInfo struct {
	// Name is the environment's name, which is the one in the file name, or
	// "default" for the base config.
	Name string
	// FilePath is the deploy config the environment was read from.
	FilePath string
	// DeployTarget is deployTarget.name.
	DeployTarget string
	// AppEnv is deployTarget.appEnv, which the deploy engine prefixes config
	// store names with. Falls back to Name where the file does not set one.
	AppEnv string
	// Region and Profile are the cloud settings worth showing next to an
	// environment when choosing between them, pulled from deployTarget.config.
	Region  string
	Profile string
}

// DeployConfigResult is what a deploy config file says about its environment.
type DeployConfigResult struct {
	DeployTarget string
	AppEnv       string
	// Config is deployTarget.config with its values rendered as strings.
	Config map[string]string
}

// defaultEnvName is the environment of the base deploy config, which names no
// environment of its own.
const defaultEnvName = "default"

// envConfigPrefix and envConfigSuffixes bracket an environment name in a
// per-environment deploy config file name.
const envConfigPrefix = "app.deploy."

var envConfigSuffixes = []string{".jsonc", ".json"}

// ReadDeployConfig reads the deploy target settings from a deploy config file.
func ReadDeployConfig(path string) (*DeployConfigResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading deploy config %s: %w", path, err)
	}

	var cfg struct {
		DeployTarget struct {
			Name   string         `json:"name"`
			AppEnv string         `json:"appEnv"`
			Config map[string]any `json:"config"`
		} `json:"deployTarget"`
	}
	if err := json.Unmarshal([]byte(stripJSONCComments(string(data))), &cfg); err != nil {
		return nil, fmt.Errorf("parsing deploy config %s: %w", path, err)
	}

	return &DeployConfigResult{
		DeployTarget: cfg.DeployTarget.Name,
		AppEnv:       cfg.DeployTarget.AppEnv,
		Config:       stringValues(cfg.DeployTarget.Config),
	}, nil
}

// FindEnvDeployConfig returns the path of the deploy config for a named
// environment, or an empty string where the environment has no file of its own.
func FindEnvDeployConfig(appDir, envName string) string {
	for _, suffix := range envConfigSuffixes {
		path := filepath.Join(appDir, envConfigPrefix+envName+suffix)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// DiscoverEnvironments returns every environment the application declares, in
// name order with the base config first.
//
// The base config is listed as "default" rather than left out. It is the one
// an application starts with and often the only one, so an application with no
// per-environment files would otherwise have nothing to choose from.
func DiscoverEnvironments(appDir string) ([]EnvironmentInfo, error) {
	entries, err := os.ReadDir(appDir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", appDir, err)
	}

	var envs []EnvironmentInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name, ok := environmentNameOf(entry.Name())
		if !ok {
			continue
		}
		info, err := readEnvironment(filepath.Join(appDir, entry.Name()), name)
		if err != nil {
			// A file that does not parse is reported rather than skipped: it
			// is an environment the author meant to have, and silently
			// omitting it reads as the environment not existing.
			return nil, err
		}
		envs = append(envs, *info)
	}

	sort.Slice(envs, func(i, j int) bool {
		// The base config first, then by name, so the list is stable and the
		// one an application always has is the one it opens on.
		if (envs[i].Name == defaultEnvName) != (envs[j].Name == defaultEnvName) {
			return envs[i].Name == defaultEnvName
		}
		return envs[i].Name < envs[j].Name
	})
	return envs, nil
}

// environmentNameOf reports the environment a deploy config file name belongs
// to.
func environmentNameOf(fileName string) (string, bool) {
	for _, suffix := range envConfigSuffixes {
		if fileName == "app.deploy"+suffix {
			return defaultEnvName, true
		}
		if strings.HasPrefix(fileName, envConfigPrefix) && strings.HasSuffix(fileName, suffix) {
			name := strings.TrimSuffix(strings.TrimPrefix(fileName, envConfigPrefix), suffix)
			if name != "" {
				return name, true
			}
		}
	}
	return "", false
}

func readEnvironment(path, name string) (*EnvironmentInfo, error) {
	result, err := ReadDeployConfig(path)
	if err != nil {
		return nil, err
	}

	appEnv := result.AppEnv
	if appEnv == "" {
		// The deploy engine prefixes config store names with appEnv, so an
		// environment without one would share its stores with every other.
		appEnv = name
	}

	return &EnvironmentInfo{
		Name:         name,
		FilePath:     path,
		DeployTarget: result.DeployTarget,
		AppEnv:       appEnv,
		Region:       result.Config["aws.region"],
		Profile:      result.Config["aws.profile"],
	}, nil
}

// stringValues renders a deploy target's config as strings.
//
// The field is free-form, so a value that is written unquoted in the file
// arrives as a number or a bool. Rendering rather than dropping those keeps a
// region or profile readable whatever it was written as.
func stringValues(config map[string]any) map[string]string {
	if len(config) == 0 {
		return nil
	}
	out := make(map[string]string, len(config))
	for key, value := range config {
		switch v := value.(type) {
		case string:
			out[key] = v
		case bool:
			out[key] = strconv.FormatBool(v)
		case float64:
			out[key] = strconv.FormatFloat(v, 'f', -1, 64)
		case nil:
			// Absent rather than the string "null", so a caller's empty check
			// means what it looks like.
		default:
			out[key] = fmt.Sprintf("%v", v)
		}
	}
	return out
}

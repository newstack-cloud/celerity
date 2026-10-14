package devconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
)

// deployConfig represents the minimal structure of app.deploy.jsonc.
//
// Read here rather than through internal/deployconfig so that a dev session
// does not depend on the deploy engine's configuration conversion, which
// needs far more of the file than a dev run does.
type deployConfig struct {
	DeployTarget struct {
		Name string `json:"name"`
	} `json:"deployTarget"`
	HandlerBuild *handlertoolchain.Config `json:"handlerBuild,omitempty"`
}

// ReadDeployTarget reads the deploy target name from a deploy config file.
// Supports JSONC (strips comments before parsing).
func ReadDeployTarget(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading deploy config %s: %w", path, err)
	}

	cleaned := stripJSONCComments(string(data))

	var cfg deployConfig
	if err := json.Unmarshal([]byte(cleaned), &cfg); err != nil {
		return "", fmt.Errorf("parsing deploy config %s: %w", path, err)
	}

	if cfg.DeployTarget.Name == "" {
		return "", fmt.Errorf("deploy config %s missing deployTarget.name", path)
	}

	return cfg.DeployTarget.Name, nil
}

// FindDeployConfig looks for a deploy config file in the given directory.
// Returns the path if found, empty string if not.
func FindDeployConfig(appDir string) string {
	candidates := []string{"app.deploy.jsonc", "app.deploy.json"}
	for _, name := range candidates {
		path := appDir + "/" + name
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

var (
	lineCommentRe  = regexp.MustCompile(`//.*`)
	blockCommentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)
)

// stripJSONCComments removes single-line and block comments from JSONC.
// Also removes trailing commas before } or ].
func stripJSONCComments(s string) string {
	s = blockCommentRe.ReplaceAllString(s, "")
	s = lineCommentRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, ",\n}", "\n}")
	s = strings.ReplaceAll(s, ",\n]", "\n]")
	return s
}

// ReadHandlerBuild reads the handlerBuild block from a deploy config file,
// returning nil where the file declares none.
//
// How a community SDK's handlers executable is built, which for an OS-only
// runtime is the only thing that says what language the application is in.
func ReadHandlerBuild(path string) (*handlertoolchain.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading deploy config %s: %w", path, err)
	}

	var cfg deployConfig
	if err := json.Unmarshal([]byte(stripJSONCComments(string(data))), &cfg); err != nil {
		return nil, fmt.Errorf("parsing deploy config %s: %w", path, err)
	}
	if cfg.HandlerBuild == nil {
		return nil, nil
	}
	if err := cfg.HandlerBuild.Validate(); err != nil {
		return nil, fmt.Errorf("deploy config %s: %w", path, err)
	}
	return cfg.HandlerBuild, nil
}

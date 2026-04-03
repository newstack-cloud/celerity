package devconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// deployConfig represents the minimal structure of app.deploy.jsonc.
type deployConfig struct {
	DeployTarget struct {
		Name string `json:"name"`
	} `json:"deployTarget"`
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

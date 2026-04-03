package blueprint

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/newstack-cloud/celerity/apps/cli/internal/consts"
)

const nodeSDKPackage = "@celerity-sdk/core"
const pythonSDKPackage = "celerity-sdk"

// DetectSDKVersion reads the project's dependency file and returns the
// Celerity SDK version declared for the given runtime.
// The returned version is stripped of range prefixes (^, ~, >=, etc.)
// so it can be used directly as a Docker image tag component.
func DetectSDKVersion(appDir string, runtime string) (string, error) {
	for prefix, detect := range sdkDetectors {
		if strings.HasPrefix(runtime, prefix) {
			return detect(appDir)
		}
	}
	return "", fmt.Errorf("no SDK version detection for runtime %q", runtime)
}

var sdkDetectors = map[string]func(string) (string, error){
	consts.LanguageNodeJS: detectNodeSDKVersion,
	consts.LanguagePython: detectPythonSDKVersion,
}

// nodePackageJSON is the minimal structure needed to extract dependency versions.
type nodePackageJSON struct {
	PeerDependencies map[string]string `json:"peerDependencies"`
	Dependencies     map[string]string `json:"dependencies"`
	DevDependencies  map[string]string `json:"devDependencies"`
}

func detectNodeSDKVersion(appDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(appDir, "package.json"))
	if err != nil {
		return "", fmt.Errorf("reading package.json: %w", err)
	}

	var pkg nodePackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("parsing package.json: %w", err)
	}

	// Check in priority order: peerDependencies → dependencies → devDependencies.
	for _, deps := range []map[string]string{
		pkg.PeerDependencies,
		pkg.Dependencies,
		pkg.DevDependencies,
	} {
		if v, ok := deps[nodeSDKPackage]; ok && v != "" {
			return stripVersionPrefix(v), nil
		}
	}

	return "", fmt.Errorf("%s not found in package.json dependencies", nodeSDKPackage)
}

// pythonVersionRe matches a PEP 508 dependency like "celerity-sdk>=0.3.1" or "celerity-sdk[cli]>=0.3.1".
var pythonVersionRe = regexp.MustCompile(
	`(?i)^["']?` + regexp.QuoteMeta(pythonSDKPackage) + `(?:\[.*?\])?([><=!~]+)([\d][^\s"',]*)`,
)

func detectPythonSDKVersion(appDir string) (string, error) {
	f, err := os.Open(filepath.Join(appDir, "pyproject.toml"))
	if err != nil {
		return "", fmt.Errorf("reading pyproject.toml: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if m := pythonVersionRe.FindStringSubmatch(line); m != nil {
			return m[2], nil
		}
	}

	return "", fmt.Errorf("%s not found in pyproject.toml dependencies", pythonSDKPackage)
}

// stripVersionPrefix removes common semver range prefixes.
func stripVersionPrefix(version string) string {
	return strings.TrimLeft(version, "^~>=!v ")
}

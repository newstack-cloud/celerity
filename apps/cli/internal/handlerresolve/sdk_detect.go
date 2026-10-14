package handlerresolve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RequireCelerityRuntimeSDK fails fast at build time with a clear message
// instead of letting the generated bootstrap fail at Lambda runtime when
// the Celerity SDK isn't installed.
func RequireCelerityRuntimeSDK(appDir, runtime string) error {
	switch {
	case isNodeRuntime(runtime):
		return requireNodeSDK(appDir)
	case isPythonRuntime(runtime):
		return requirePythonSDK(appDir)
	}
	return nil
}

func requireNodeSDK(appDir string) error {
	if nodeHasCelerityDep(appDir) {
		return nil
	}
	return fmt.Errorf(
		"celerity/handler resources declared but no @celerity-sdk/* package is a dependency " +
			"in package.json; add @celerity-sdk/core (and @celerity-sdk/serverless-aws for " +
			"Lambda deploys) to your dependencies",
	)
}

func requirePythonSDK(appDir string) error {
	if pythonHasCelerityDep(appDir) {
		return nil
	}
	return fmt.Errorf(
		"celerity/handler resources declared but celerity-sdk is not a dependency in " +
			"pyproject.toml; add celerity-sdk to your project dependencies",
	)
}

func isNodeRuntime(runtime string) bool {
	return strings.HasPrefix(runtime, "nodejs")
}

func isPythonRuntime(runtime string) bool {
	return strings.HasPrefix(runtime, "python")
}

func nodeHasCelerityDep(appDir string) bool {
	data, err := os.ReadFile(filepath.Join(appDir, "package.json"))
	if err != nil {
		return false
	}
	var pkg struct {
		Dependencies     map[string]string `json:"dependencies"`
		DevDependencies  map[string]string `json:"devDependencies"`
		PeerDependencies map[string]string `json:"peerDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	return hasCelerityScopedPackage(pkg.Dependencies) ||
		hasCelerityScopedPackage(pkg.DevDependencies) ||
		hasCelerityScopedPackage(pkg.PeerDependencies)
}

func hasCelerityScopedPackage(deps map[string]string) bool {
	for name := range deps {
		if strings.HasPrefix(name, "@celerity-sdk/") {
			return true
		}
	}
	return false
}

// A line-based scan instead of full TOML parsing, to avoid pulling in a parser
// dependency for a simple substring check.
func pythonHasCelerityDep(appDir string) bool {
	data, err := os.ReadFile(filepath.Join(appDir, "pyproject.toml"))
	if err != nil {
		return false
	}
	return bytesContainCelerityDep(data)
}

func bytesContainCelerityDep(data []byte) bool {
	text := string(data)
	for line := range strings.SplitSeq(text, "\n") {
		stripped := strings.TrimSpace(line)
		if lineDeclaresCelerityDep(stripped) {
			return true
		}
	}
	return false
}

// Recognises the common pyproject.toml formats:
//   - `celerity-sdk = "^1.0"` (poetry)
//   - `"celerity-sdk>=1.0",` (PEP 621 list entry)
//   - `celerity-sdk` (bare name)
func lineDeclaresCelerityDep(line string) bool {
	if line == "" || strings.HasPrefix(line, "#") {
		return false
	}
	lowered := strings.ToLower(line)
	idx := strings.Index(lowered, "celerity-sdk")
	if idx == -1 {
		return false
	}
	if !hasPackageNameBoundaryBefore(lowered, idx) {
		return false
	}
	return hasPackageNameBoundaryAfter(lowered, idx+len("celerity-sdk"))
}

func hasPackageNameBoundaryBefore(s string, idx int) bool {
	if idx == 0 {
		return true
	}
	ch := s[idx-1]
	return ch == '"' || ch == '\'' || ch == '[' || ch == ' ' || ch == '\t' || ch == ','
}

func hasPackageNameBoundaryAfter(s string, idx int) bool {
	if idx >= len(s) {
		return true
	}
	ch := s[idx]
	return ch == '"' || ch == '\'' || ch == ' ' || ch == '\t' ||
		ch == '=' || ch == '>' || ch == '<' || ch == '~' ||
		ch == ',' || ch == ']'
}

// RequireRuntimeSDK checks that the project declares whatever it needs to
// build and run its handlers.
//
// Prefer this over RequireCelerityRuntimeSDK. For an OS-only runtime the
// question is not whether a language SDK is installed but whether the resolved
// toolchain can build an executable at all, which only that toolchain can
// report. RequireCelerityRuntimeSDK returns nil for a runtime it does not
// recognise, so calling it directly for an OS-only project checks nothing.
func RequireRuntimeSDK(appDir string, resolved *ResolvedHandlers) error {
	if resolved == nil {
		return nil
	}
	if resolved.Toolchain != nil {
		return resolved.Toolchain.Validate(appDir)
	}
	return RequireCelerityRuntimeSDK(appDir, resolved.Runtime)
}

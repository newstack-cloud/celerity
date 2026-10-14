package handlertoolchain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// A text scan rather than a go.mod parse. All that is asked of the file is
// whether a module path appears as a requirement, whether it appears as a
// tool, and what version a requirement names, none of which need the file
// understood, and reaching for golang.org/x/mod would pull a dependency into
// a package that otherwise needs none.

func readGoMod(appDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(appDir, "go.mod"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading go.mod: %w", err)
	}
	return string(data), nil
}

// A path that merely starts with the one asked for does not count, so a
// hypothetical celerity-go-sdk-contrib is not mistaken for the SDK.
func requires(goMod, modulePath string) (version string, found bool) {
	for line := range strings.Lines(goMod) {
		rest, ok := directiveArgument(line, modulePath)
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return "", true
		}
		return fields[0], true
	}
	return "", false
}

// A tool dependency is what makes `go tool <name>` resolve a module. Both
// spellings count, a lone `tool <path>` and a path inside a `tool (...)`
// block.
func declaresTool(goMod, modulePath string) bool {
	inBlock := false
	for line := range strings.Lines(goMod) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		switch {
		case trimmed == "tool (":
			inBlock = true
			continue
		case inBlock && trimmed == ")":
			inBlock = false
			continue
		}

		candidate := trimmed
		if !inBlock {
			after, isTool := strings.CutPrefix(trimmed, "tool ")
			if !isTool {
				continue
			}
			candidate = strings.TrimSpace(after)
		}
		if candidate == modulePath {
			return true
		}
	}
	return false
}

// Returns the remainder of a line that names a module path, having rejected
// comments and longer paths that merely begin with it.
func directiveArgument(line, modulePath string) (rest string, found bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "//") {
		return "", false
	}
	_, after, ok := strings.Cut(trimmed, modulePath)
	if !ok {
		return "", false
	}
	if after != "" && isModulePathChar(after[0]) {
		return "", false
	}
	return strings.TrimSpace(after), true
}

func isModulePathChar(c byte) bool {
	return c == '-' || c == '.' || c == '_' || c == '/' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

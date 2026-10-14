package preprocess

import (
	"fmt"
	"strings"
)

// ManifestVersion is the handler manifest format this package reads.
//
// Accepted in two spellings. The Celerity Go SDK emits "v1" while the CLI's
// own no-extraction fallback constructs "1", and both have been in the wild
// unvalidated. Rejecting either now would break working projects to enforce a
// distinction that never meant anything.
const (
	ManifestVersion      = "1"
	ManifestVersionGoSDK = "v1"
)

// ValidateManifest reports whether a handler manifest can be merged into a
// blueprint.
//
// Worth checking for an extractor the CLI does not ship. A community SDK
// hand-produces this document and the merge is forgiving by design, skipping
// what it does not recognise. A manifest with the wrong annotation keys, or
// handlers with no resource name, therefore merges cleanly and produces an
// application with no handlers at all, surfacing as a runtime handshake
// failure a long way from the mistake.
func ValidateManifest(m *HandlerManifest) error {
	if m == nil {
		return fmt.Errorf("no manifest")
	}

	switch m.Version {
	case ManifestVersion, ManifestVersionGoSDK:
	case "":
		return fmt.Errorf(
			"the manifest declares no version; this CLI reads %q",
			ManifestVersion,
		)
	default:
		return fmt.Errorf(
			"the manifest declares version %q and this CLI reads %q",
			m.Version, ManifestVersion,
		)
	}

	var problems []string
	for i, h := range m.Handlers {
		problems = append(problems, handlerProblems(fmt.Sprintf("handlers[%d]", i), h.ResourceName, h.Annotations)...)
	}
	for i, h := range m.FunctionHandlers {
		problems = append(problems, handlerProblems(fmt.Sprintf("functionHandlers[%d]", i), h.ResourceName, h.Annotations)...)
	}
	for i, h := range m.GuardHandlers {
		problems = append(problems, handlerProblems(fmt.Sprintf("guardHandlers[%d]", i), h.ResourceName, h.Annotations)...)
	}

	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "\n"))
	}
	return nil
}

func handlerProblems(where, resourceName string, annotations map[string]any) []string {
	var problems []string
	if resourceName == "" {
		problems = append(problems, fmt.Sprintf(
			"%s has no resourceName, which is what names it in the blueprint", where))
	}
	for key := range annotations {
		if !strings.HasPrefix(key, annotationPrefix) {
			problems = append(problems, fmt.Sprintf(
				"%s has annotation %q, which is not a Celerity annotation: every key starts with %q",
				where, key, annotationPrefix))
		}
	}
	return problems
}

const annotationPrefix = "celerity."

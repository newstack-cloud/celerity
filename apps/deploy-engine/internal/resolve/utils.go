package resolve

import (
	"fmt"
	"strings"
)

// BlueprintLocationString returns a string representation of the
// blueprint location based on the provided BlueprintDocumentInfo
// that represents blueprint location information derived from a request
// payload.
func BlueprintLocationString(
	payload *BlueprintDocumentInfo,
) string {
	// Normalise to use forward slashes to provide a consistent
	// location format for different platforms and remote locations.
	// This is not used to resolve the location, only to provide a handle
	// to the location for the client where the client is expected to have
	// the additional context.
	directory := strings.TrimSuffix(payload.Directory, "/")
	blueprintFile := strings.TrimPrefix(
		payload.BlueprintFile,
		"/",
	)

	return fmt.Sprintf(
		"%s://%s/%s",
		payload.FileSourceScheme,
		directory,
		blueprintFile,
	)
}

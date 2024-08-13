package validation

import (
	"context"
	"fmt"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
)

// ValidateInclude deals with early stage validation of a child blueprint
// include. This validation is primarily responsible for ensuring the
// path of an include is not empty.
// As we don't have enough extra information at the early stage at which this should run,
// it does not include validation of the path format or variables.
// Variable validation requires information about the variables that are available
// in the child blueprint, which is not available at this stage.
func ValidateInclude(
	ctx context.Context,
	includeName string,
	includeSchema *schema.Include,
	includeMap *schema.IncludeMap,
) ([]*core.Diagnostic, error) {
	diagnostics := []*core.Diagnostic{}
	includeSubContext := fmt.Sprintf("include.%s", includeName)

	formatted, err := substitutions.SubstitutionsToString(includeSubContext, includeSchema.Path)
	if err != nil {
		return diagnostics, err
	}

	return diagnostics, validatePathFormat(includeName, formatted, includeMap)
}

func validatePathFormat(includeName, path string, includeMap *schema.IncludeMap) error {
	if strings.TrimSpace(path) == "" {
		return errIncludeEmptyPath(includeName, getIncludeSourceMeta(includeMap, includeName))
	}

	// Beyond checking if it is empty,
	// there is no need to validate the path at this stage as it will be sanitised
	// as a part of path processing by the include file resolver.
	// The include file resolver will report issues with the path.

	return nil
}

func getIncludeSourceMeta(includeMap *schema.IncludeMap, varName string) *source.Meta {
	if includeMap == nil {
		return nil
	}

	return includeMap.SourceMeta[varName]
}

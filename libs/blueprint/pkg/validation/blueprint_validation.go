package validation

import (
	"context"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/common/pkg/core"
)

// ValidateBlueprint ensures that the required top-level properties
// of a blueprint are populated.
// (When they are populated the schema takes care of the structure)
func ValidateBlueprint(ctx context.Context, blueprint *schema.Blueprint) error {
	if strings.TrimSpace(blueprint.Version) == "" {
		return errBlueprintMissingVersion()
	}

	if !core.SliceContainsComparable(SupportedVersions, blueprint.Version) {
		return errBlueprintUnsupportedVersion(blueprint.Version)
	}

	if blueprint.Resources == nil || len(blueprint.Resources.Values) == 0 {
		return errBlueprintMissingResources()
	}

	return nil
}

const (
	// Version2023_04_20 is the version of the blueprint specification
	// that is the sole version of the spec supported by the initial
	// version of the blueprint framework.
	Version2023_04_20 = "2023-04-20"
)

var (
	// SupportedVersions is the list of versions of the blueprint
	// specification that are supported by this version of the blueprint
	// framework.
	SupportedVersions = []string{
		Version2023_04_20,
	}
)

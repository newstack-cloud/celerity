package includes

import (
	"context"
	"os"

	"github.com/spf13/afero"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
)

type fsChildResolver struct {
	fs afero.Fs
}

// NewFileSystemChildResolver creates a new instance of a ChildResolver
// that resolves child blueprints from the provided file system.
func NewFileSystemChildResolver(fs afero.Fs) ChildResolver {
	return &fsChildResolver{
		fs,
	}
}

func (r *fsChildResolver) Resolve(
	ctx context.Context,
	includeName string,
	include *subengine.ResolvedInclude,
	params core.BlueprintParams,
) (*ChildBlueprintInfo, error) {

	// Read the child blueprint from the file system,
	// the file system is expected to be relative to the absolute root
	// path on the current system.
	path := stringValue(include.Path)
	if path == "" {
		return nil, errInvalidPath(includeName)
	}

	blueprintSource, err := afero.ReadFile(r.fs, path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errBlueprintNotFound(includeName, path)
		}
		if os.IsPermission(err) {
			return nil, errPermissions(includeName, path, err)
		}
		return nil, err
	}

	blueprintSourceStr := string(blueprintSource)
	return &ChildBlueprintInfo{
		BlueprintSource: &blueprintSourceStr,
	}, nil
}

func stringValue(value *core.MappingNode) string {
	if value == nil || value.Literal == nil || value.Literal.StringValue == nil {
		return ""
	}

	return *value.Literal.StringValue
}

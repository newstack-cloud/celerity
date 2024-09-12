package blueprint

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/transform"
)

// LoadTransformers deals with loading the providers to be used for validating
// and in providing other LSP features for blueprint such as hover information.
//
// The language server uses the build engine plugin system to load gRPC transformer
// plugins.
func LoadTransformers(ctx context.Context) (map[string]transform.SpecTransformer, error) {

	// TODO: load transformer plugins through the build engine plugin system

	return map[string]transform.SpecTransformer{}, nil
}

package engine

import (
	"context"

	"github.com/two-hundred/celerity/libs/build-engine/core"
)

// EngineAPI provides a `BuildEngine` implementation
// that uses the HTTP API to interact with the
// Build Engine.
type EngineAPI struct {
	connectProtocol string
}

func NewEngineAPI(connectProtocol string) core.BuildEngine {
	return &EngineAPI{
		connectProtocol,
	}
}

func (e *EngineAPI) Validate(ctx context.Context, params *core.ValidateParams) (*core.ValidateResults, error) {
	return nil, nil
}

func (e *EngineAPI) ValidateStream(
	ctx context.Context,
	params *core.ValidateParams,
	out chan<- *core.ValidateResult,
	errChan chan<- error,
) error {
	return nil
}

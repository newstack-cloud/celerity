package testutils

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/includes"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
)

type MockBlueprintResolver struct{}

func (m *MockBlueprintResolver) Resolve(
	ctx context.Context,
	includeName string,
	include *subengine.ResolvedInclude,
	params core.BlueprintParams,
) (*includes.ChildBlueprintInfo, error) {
	blueprintSource := "mock blueprint source"
	return &includes.ChildBlueprintInfo{
		BlueprintSource: &blueprintSource,
	}, nil
}

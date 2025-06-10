package testutils

import (
	"context"

	"github.com/newstack-cloud/celerity/libs/blueprint/core"
	"github.com/newstack-cloud/celerity/libs/blueprint/includes"
	"github.com/newstack-cloud/celerity/libs/blueprint/subengine"
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

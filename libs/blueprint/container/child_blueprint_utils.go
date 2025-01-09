package container

import (
	"context"
	"fmt"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/includes"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
)

type childBlueprintLoadResult struct {
	includeName    string
	childContainer BlueprintContainer
	childState     *state.InstanceState
	childParams    core.BlueprintParams
}

type childBlueprintLoadInput struct {
	parentInstanceID       string
	parentInstanceTreePath string
	instanceTreePath       string
	includeTreePath        string
	node                   *validation.ReferenceChainNode
	resolveFor             subengine.ResolveForStage
}

func loadChildBlueprint(
	ctx context.Context,
	input *childBlueprintLoadInput,
	substitutionResolver IncludeSubstitutionResolver,
	childResolver includes.ChildResolver,
	createChildBlueprintLoader ChildBlueprintLoaderFactory,
	stateContainer state.Container,
	paramOverrides core.BlueprintParams,
) (*childBlueprintLoadResult, error) {

	includeName := strings.TrimPrefix(input.node.ElementName, "children.")

	resolvedInclude, err := resolveIncludeForChildBlueprint(
		ctx,
		input.node,
		includeName,
		input.resolveFor,
		substitutionResolver,
	)
	if err != nil {
		return nil, err
	}

	childBlueprintInfo, err := childResolver.Resolve(ctx, includeName, resolvedInclude, paramOverrides)
	if err != nil {
		return nil, err
	}

	childParams := paramOverrides.
		WithBlueprintVariables(
			extractIncludeVariables(resolvedInclude),
			/* keepExisting */ false,
		).
		WithContextVariables(
			createContextVarsForChildBlueprint(
				input.parentInstanceID,
				input.parentInstanceTreePath,
				input.includeTreePath,
			),
			/* keepExisting */ true,
		)

	childLoader := createChildBlueprintLoader(
		/* derivedFromTemplate */ []string{},
		/* resourceTemplates */ map[string]string{},
	)

	var childContainer BlueprintContainer
	if childBlueprintInfo.AbsolutePath != nil {
		childContainer, err = childLoader.Load(ctx, *childBlueprintInfo.AbsolutePath, childParams)
		if err != nil {
			return nil, err
		}
	} else {
		format, err := extractChildBlueprintFormat(includeName, resolvedInclude)
		if err != nil {
			return nil, err
		}

		childContainer, err = childLoader.LoadString(
			ctx,
			*childBlueprintInfo.BlueprintSource,
			format,
			childParams,
		)
		if err != nil {
			return nil, err
		}
	}

	childState, err := getChildState(ctx, input.parentInstanceID, includeName, stateContainer)
	if err != nil {
		return nil, err
	}

	if hasBlueprintCycle(input.parentInstanceTreePath, childState.InstanceID) {
		return nil, errBlueprintCycleDetected(
			includeName,
			input.parentInstanceTreePath,
			childState.InstanceID,
		)
	}

	return &childBlueprintLoadResult{
		childContainer: childContainer,
		childState:     childState,
		childParams:    childParams,
		includeName:    includeName,
	}, nil
}

func getChildState(
	ctx context.Context,
	parentInstanceID string,
	includeName string,
	stateContainer state.Container,
) (*state.InstanceState, error) {
	children := stateContainer.Children()
	childState, err := children.Get(ctx, parentInstanceID, includeName)
	if err != nil {
		if !state.IsInstanceNotFound(err) {
			return nil, err
		} else {
			// Change staging includes describing the planned state for a new blueprint,
			// an empty instance ID will be used to indicate that the blueprint instance is new.
			// Deployment includes creating new blueprint instances, so an instance ID will be
			// assigned to the new blueprint instance later.
			return &state.InstanceState{
				InstanceID: "",
			}, nil
		}
	}

	return &childState, nil
}

func resolveIncludeForChildBlueprint(
	ctx context.Context,
	node *validation.ReferenceChainNode,
	includeName string,
	resolveFor subengine.ResolveForStage,
	substitutionResolver IncludeSubstitutionResolver,
) (*subengine.ResolvedInclude, error) {
	include, isInclude := node.Element.(*schema.Include)
	if !isInclude {
		return nil, fmt.Errorf("child blueprint node is not an include")
	}

	resolvedIncludeResult, err := substitutionResolver.ResolveInInclude(
		ctx,
		includeName,
		include,
		&subengine.ResolveIncludeTargetInfo{
			ResolveFor: resolveFor,
		},
	)
	if err != nil {
		return nil, err
	}

	actionText := "changes can only be staged"
	if resolveFor == subengine.ResolveForDeployment {
		actionText = "the child blueprint can only be deployed"
	}

	if len(resolvedIncludeResult.ResolveOnDeploy) > 0 {
		return nil, fmt.Errorf(
			"child blueprint include %q has unresolved substitutions, "+
				"%s for child blueprints when "+
				"all the information required to fetch and load the blueprint is available",
			node.ElementName,
			actionText,
		)
	}

	return resolvedIncludeResult.ResolvedInclude, nil
}

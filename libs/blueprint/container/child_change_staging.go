package container

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/includes"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
)

// ChildChangeStager provides an interface for a service that stages changes for
// a child blueprint included in a parent blueprint.
type ChildChangeStager interface {
	StageChanges(
		ctx context.Context,
		parentInstanceID string,
		parentInstanceTreePath string,
		includeTreePath string,
		node *validation.ReferenceChainNode,
		paramOverrides core.BlueprintParams,
		channels *ChangeStagingChannels,
	)
}

// NewDefaultLinkChangeStager creates a new instance of the default implementation
// of the service that stages changes for a child blueprint.
func NewDefaultChildChangeStager(
	childResolver includes.ChildResolver,
	createChildBlueprintLoader ChildBlueprintLoaderFactory,
	stateContainer state.Container,
	childExportFieldCache *core.Cache[*subengine.ChildExportFieldInfo],
	substitutionResolver subengine.SubstitutionResolver,
) ChildChangeStager {
	return &defaultChildChangeStager{
		childResolver:              childResolver,
		createChildBlueprintLoader: createChildBlueprintLoader,
		stateContainer:             stateContainer,
		childExportFieldCache:      childExportFieldCache,
		substitutionResolver:       substitutionResolver,
	}
}

type defaultChildChangeStager struct {
	childResolver              includes.ChildResolver
	substitutionResolver       subengine.SubstitutionResolver
	createChildBlueprintLoader ChildBlueprintLoaderFactory
	stateContainer             state.Container
	childExportFieldCache      *core.Cache[*subengine.ChildExportFieldInfo]
}

func (d *defaultChildChangeStager) StageChanges(
	ctx context.Context,
	parentInstanceID string,
	parentInstanceTreePath string,
	includeTreePath string,
	node *validation.ReferenceChainNode,
	paramOverrides core.BlueprintParams,
	channels *ChangeStagingChannels,
) {

	includeName := strings.TrimPrefix(node.ElementName, "children.")

	resolvedInclude, err := d.resolveIncludeForChildBlueprint(
		ctx,
		node,
		includeName,
	)
	if err != nil {
		channels.ErrChan <- err
		return
	}

	childBlueprintInfo, err := d.childResolver.Resolve(ctx, includeName, resolvedInclude, paramOverrides)
	if err != nil {
		channels.ErrChan <- err
		return
	}

	childParams := paramOverrides.
		WithBlueprintVariables(
			extractIncludeVariables(resolvedInclude),
			/* keepExisting */ false,
		).
		WithContextVariables(
			createContextVarsForChildBlueprint(
				parentInstanceID,
				parentInstanceTreePath,
				includeTreePath,
			),
			/* keepExisting */ true,
		)

	childLoader := d.createChildBlueprintLoader(
		/* derivedFromTemplate */ []string{},
		/* resourceTemplates */ map[string]string{},
	)

	var childContainer BlueprintContainer
	if childBlueprintInfo.AbsolutePath != nil {
		childContainer, err = childLoader.Load(ctx, *childBlueprintInfo.AbsolutePath, childParams)
		if err != nil {
			channels.ErrChan <- err
			return
		}
	} else {
		format, err := extractChildBlueprintFormat(includeName, resolvedInclude)
		if err != nil {
			channels.ErrChan <- err
			return
		}

		childContainer, err = childLoader.LoadString(
			ctx,
			*childBlueprintInfo.BlueprintSource,
			format,
			childParams,
		)
		if err != nil {
			channels.ErrChan <- err
			return
		}
	}

	childState, err := d.getChildState(ctx, parentInstanceID, includeName)
	if err != nil {
		channels.ErrChan <- err
		return
	}

	if hasBlueprintCycle(parentInstanceTreePath, childState.InstanceID) {
		channels.ErrChan <- errBlueprintCycleDetected(
			includeName,
			parentInstanceTreePath,
			childState.InstanceID,
		)
		return
	}

	childChannels := &ChangeStagingChannels{
		ResourceChangesChan: make(chan ResourceChangesMessage),
		ChildChangesChan:    make(chan ChildChangesMessage),
		LinkChangesChan:     make(chan LinkChangesMessage),
		CompleteChan:        make(chan BlueprintChanges),
		ErrChan:             make(chan error),
	}
	err = childContainer.StageChanges(
		ctx,
		&StageChangesInput{
			InstanceID: childState.InstanceID,
		},
		childChannels,
		childParams,
	)
	if err != nil {
		channels.ErrChan <- err
		return
	}

	d.waitForChildChanges(ctx, includeName, childState, childChannels, channels)
}

func (d *defaultChildChangeStager) resolveIncludeForChildBlueprint(
	ctx context.Context,
	node *validation.ReferenceChainNode,
	includeName string,
) (*subengine.ResolvedInclude, error) {
	include, isInclude := node.Element.(*schema.Include)
	if !isInclude {
		return nil, fmt.Errorf("child blueprint node is not an include")
	}

	resolvedIncludeResult, err := d.substitutionResolver.ResolveInInclude(
		ctx,
		includeName,
		include,
		&subengine.ResolveIncludeTargetInfo{
			ResolveFor: subengine.ResolveForChangeStaging,
		},
	)
	if err != nil {
		return nil, err
	}

	if len(resolvedIncludeResult.ResolveOnDeploy) > 0 {
		return nil, fmt.Errorf(
			"child blueprint include %q has unresolved substitutions, "+
				"changes can only be staged for child blueprints when "+
				"all the information required to fetch and load the blueprint is available",
			node.ElementName,
		)
	}

	return resolvedIncludeResult.ResolvedInclude, nil
}

func (d *defaultChildChangeStager) getChildState(
	ctx context.Context,
	parentInstanceID string,
	includeName string,
) (*state.InstanceState, error) {
	children := d.stateContainer.Children()
	childState, err := children.Get(ctx, parentInstanceID, includeName)
	if err != nil {
		if !state.IsInstanceNotFound(err) {
			return nil, err
		} else {
			// Change staging includes describing the planned state for a new blueprint,
			// an empty instance ID will be used to indicate that the blueprint instance is new.
			return &state.InstanceState{
				InstanceID: "",
			}, nil
		}
	}

	return &childState, nil
}

func (d *defaultChildChangeStager) waitForChildChanges(
	ctx context.Context,
	includeName string,
	childState *state.InstanceState,
	childChannels *ChangeStagingChannels,
	channels *ChangeStagingChannels,
) {
	// For now, when it comes to child blueprints,
	// wait for all changes to be staged before sending
	// an update message for the parent blueprint context.
	// In the future, we may want to stream changes
	// in child blueprints like with resources and links
	// in the parent blueprint.
	var changes BlueprintChanges
	receivedFullChildChanges := false
	var stagingErr error
	for !receivedFullChildChanges && stagingErr == nil {
		select {
		case <-ctx.Done():
			stagingErr = ctx.Err()
		case <-childChannels.ResourceChangesChan:
		case <-childChannels.LinkChangesChan:
		case <-childChannels.ChildChangesChan:
		case changes = <-childChannels.CompleteChan:
			d.cacheChildExportFields(includeName, &changes)
			channels.ChildChangesChan <- ChildChangesMessage{
				ChildBlueprintName: includeName,
				Removed:            false,
				New:                childState.InstanceID == "",
				Changes:            changes,
			}
		case stagingErr = <-childChannels.ErrChan:
			channels.ErrChan <- stagingErr
		}
	}
}

func (d *defaultChildChangeStager) cacheChildExportFields(
	childName string,
	changes *BlueprintChanges,
) {
	for exportName, fieldChange := range changes.ExportChanges {
		d.cacheChildExportField(
			childName,
			changes,
			exportName,
			fieldChange,
		)
	}

	for exportName, fieldChange := range changes.NewExports {
		d.cacheChildExportField(
			childName,
			changes,
			exportName,
			fieldChange,
		)
	}

	for _, exportName := range changes.RemovedExports {
		key := substitutions.RenderFieldPath(childName, exportName)
		d.childExportFieldCache.Set(
			key,
			&subengine.ChildExportFieldInfo{
				Value:           nil,
				Removed:         true,
				ResolveOnDeploy: false,
			},
		)
	}
}

func (d *defaultChildChangeStager) cacheChildExportField(
	childName string,
	changes *BlueprintChanges,
	exportName string,
	fieldChange provider.FieldChange,
) {
	key := substitutions.RenderFieldPath(childName, exportName)
	exportFieldPath := substitutions.RenderFieldPath("exports", exportName)
	willResolveOnDeploy := slices.Contains(
		changes.ResolveOnDeploy,
		exportFieldPath,
	)

	d.childExportFieldCache.Set(
		key,
		&subengine.ChildExportFieldInfo{
			Value:           fieldChange.NewValue,
			Removed:         false,
			ResolveOnDeploy: willResolveOnDeploy,
		},
	)
}

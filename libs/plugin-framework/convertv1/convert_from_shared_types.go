package convertv1

import (
	"errors"
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/schemapb"
	"github.com/two-hundred/celerity/libs/blueprint/serialisation"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
	"github.com/two-hundred/celerity/libs/plugin-framework/pbutils"
	"github.com/two-hundred/celerity/libs/plugin-framework/sharedtypesv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/utils"
)

// FromPBScalarMap converts a map of protobuf ScalarValues to a map of core ScalarValues
// compatible with the blueprint framework.
func FromPBScalarMap(m map[string]*schemapb.ScalarValue) (map[string]*core.ScalarValue, error) {
	coreMap := make(map[string]*core.ScalarValue)
	for k, scalar := range m {
		coreScalar, err := serialisation.FromScalarValuePB(scalar, true /* optional */)
		if err != nil {
			return nil, err
		}
		coreMap[k] = coreScalar
	}
	return coreMap, nil
}

// FromPBFunctionDefinitionRequest converts a FunctionDefinitionRequest from a protobuf message to a core type
// compatible with the blueprint framework.
func FromPBFunctionDefinitionRequest(
	req *sharedtypesv1.FunctionDefinitionRequest,
) (*provider.FunctionGetDefinitionInput, error) {
	params, err := fromPBBlueprintParams(req.Params)
	if err != nil {
		return nil, err
	}

	return &provider.FunctionGetDefinitionInput{
		Params: params,
	}, nil
}

// FromPBFunctionCallRequest converts a FunctionCallRequest from a protobuf message to a core type
// compatible with the blueprint framework.
func FromPBFunctionCallRequest(
	req *sharedtypesv1.FunctionCallRequest,
	functionRegistry provider.FunctionRegistry,
) (*provider.FunctionCallInput, error) {
	deserialisedArgs, err := fromPBFunctionCallArgs(req.Args)
	if err != nil {
		return nil, err
	}

	callCtxInfo, err := fromPBFunctionCallContext(req.CallContext)
	if err != nil {
		return nil, err
	}
	// Ensure the function registry is scoped to the current call context
	// based on the stack received over the "wire".
	scopedFunctionRegistry := functionRegistry.ForCallContext(callCtxInfo.stack)
	callCtx := subengine.NewFunctionCallContext(
		callCtxInfo.stack,
		scopedFunctionRegistry,
		callCtxInfo.params,
		callCtxInfo.location,
	)
	args := callCtx.NewCallArgs(deserialisedArgs...)
	callCtx.NewCallArgs()

	return &provider.FunctionCallInput{
		Arguments:   args,
		CallContext: callCtx,
	}, nil
}

// FromPBFunctionCallResult converts a FunctionCallResult from a protobuf message to a core type
// compatible with the blueprint framework.
func FromPBFunctionCallResult(
	res *sharedtypesv1.FunctionCallResult,
) (*provider.FunctionCallOutput, error) {
	if res == nil {
		return nil, nil
	}

	responseData, err := pbutils.ConvertPBAnyToInterface(res.ResponseData)
	if err != nil {
		return nil, err
	}

	funcInfo, err := fromPBFunctionRuntimeInfo(res.FunctionInfo)
	if err != nil {
		return nil, err
	}

	return &provider.FunctionCallOutput{
		ResponseData: responseData,
		FunctionInfo: funcInfo,
	}, nil
}

func fromPBFunctionRuntimeInfo(
	pbFuncInfo *sharedtypesv1.FunctionRuntimeInfo,
) (provider.FunctionRuntimeInfo, error) {
	if pbFuncInfo == nil {
		return provider.FunctionRuntimeInfo{}, nil
	}

	partialArgs, err := pbutils.ConvertPBAnyToInterface(pbFuncInfo.PartialArgs)
	if err != nil {
		return provider.FunctionRuntimeInfo{}, err
	}

	partialArgsSlice, ok := partialArgs.([]any)
	if !ok {
		return provider.FunctionRuntimeInfo{}, fmt.Errorf(
			"expected partial args to be a []any, got %T",
			partialArgs,
		)
	}

	return provider.FunctionRuntimeInfo{
		FunctionName: pbFuncInfo.FunctionName,
		PartialArgs:  partialArgsSlice,
		ArgsOffset:   int(pbFuncInfo.ArgsOffset),
	}, nil
}

// FromPBFunctionDefinition converts a FunctionDefinition from a protobuf message to a core type
// compatible with the blueprint framework.
func FromPBFunctionDefinition(
	pbFuncDef *sharedtypesv1.FunctionDefinition,
) (*function.Definition, error) {
	if pbFuncDef == nil {
		return nil, nil
	}

	return &function.Definition{
		Description:          pbFuncDef.Description,
		FormattedDescription: pbFuncDef.FormattedDescription,
		// TODO: Parameters
		// TODO: Return
		Internal: pbFuncDef.Internal,
	}, nil
}

// FromPBDeployResourceRequest converts a DeployResourceRequest from a protobuf message to a core type
// compatible with the blueprint framework.
func FromPBDeployResourceRequest(
	req *sharedtypesv1.DeployResourceRequest,
) (*provider.ResourceDeployInput, error) {
	changes, err := fromPBResourceChanges(req.Changes)
	if err != nil {
		return nil, err
	}

	providerCtx, err := FromPBProviderContext(req.Context)
	if err != nil {
		return nil, err
	}

	return &provider.ResourceDeployInput{
		InstanceID:      req.InstanceId,
		ResourceID:      req.ResourceId,
		Changes:         changes,
		ProviderContext: providerCtx,
	}, nil
}

func fromPBResourceChanges(changes *sharedtypesv1.Changes) (*provider.Changes, error) {
	if changes == nil {
		return nil, nil
	}

	appliedResourceInfo, err := fromPBResourceInfo(changes.AppliedResourceInfo)
	if err != nil {
		return nil, err
	}

	modifiedFields, err := fromPBFieldChanges(changes.ModifiedFields)
	if err != nil {
		return nil, err
	}

	newFields, err := fromPBFieldChanges(changes.NewFields)
	if err != nil {
		return nil, err
	}

	newOutboundLinks, err := fromPBLinkChangesMap(changes.NewOutboundLinks)
	if err != nil {
		return nil, err
	}

	outboundLinkChanges, err := fromPBLinkChangesMap(changes.OutboundLinkChanges)
	if err != nil {
		return nil, err
	}

	return &provider.Changes{
		AppliedResourceInfo:       appliedResourceInfo,
		MustRecreate:              changes.MustRecreate,
		ModifiedFields:            modifiedFields,
		NewFields:                 newFields,
		RemovedFields:             changes.RemovedFields,
		UnchangedFields:           changes.UnchangedFields,
		ComputedFields:            changes.ComputedFields,
		FieldChangesKnownOnDeploy: changes.FieldChangesKnownOnDeploy,
		ConditionKnownOnDeploy:    changes.ConditionKnownOnDeploy,
		NewOutboundLinks:          newOutboundLinks,
		OutboundLinkChanges:       outboundLinkChanges,
		RemovedOutboundLinks:      changes.RemovedOutboundLinks,
	}, nil
}

func fromPBResourceInfo(
	resourceInfo *sharedtypesv1.ResourceInfo,
) (provider.ResourceInfo, error) {
	if resourceInfo == nil {
		return provider.ResourceInfo{}, nil
	}

	resourceState, err := fromPBResourceState(resourceInfo.CurrentResourceState)
	if err != nil {
		return provider.ResourceInfo{}, err
	}

	resourceWithResolvedSubs, err := fromPBResolvedResource(resourceInfo.ResourceWithResolvedSubs)
	if err != nil {
		return provider.ResourceInfo{}, err
	}

	return provider.ResourceInfo{
		ResourceID:               resourceInfo.ResourceId,
		ResourceName:             resourceInfo.ResourceName,
		InstanceID:               resourceInfo.InstanceId,
		CurrentResourceState:     resourceState,
		ResourceWithResolvedSubs: resourceWithResolvedSubs,
	}, nil
}

func fromPBResolvedResource(
	resolvedResource *sharedtypesv1.ResolvedResource,
) (*provider.ResolvedResource, error) {
	if resolvedResource == nil {
		return nil, nil
	}

	description, err := serialisation.FromMappingNodePB(
		resolvedResource.Description,
		/* optional */ true,
	)
	if err != nil {
		return nil, err
	}

	resolvedResourceMetadata, err := fromPBResolvedResourceMetadata(
		resolvedResource.Metadata,
	)
	if err != nil {
		return nil, err
	}

	spec, err := serialisation.FromMappingNodePB(
		resolvedResource.Spec,
		/* optional */ true,
	)
	if err != nil {
		return nil, err
	}

	return &provider.ResolvedResource{
		Type: &schema.ResourceTypeWrapper{
			Value: ResourceTypeToString(resolvedResource.Type),
		},
		Description: description,
		Metadata:    resolvedResourceMetadata,
		// TODO: Condition
		// TODO: LinkSelector
		Spec: spec,
	}, nil
}

func fromPBResolvedResourceMetadata(
	pbResourceMetadata *sharedtypesv1.ResolvedResourceMetadata,
) (*provider.ResolvedResourceMetadata, error) {
	if pbResourceMetadata == nil {
		return nil, nil
	}

	displayName, err := serialisation.FromMappingNodePB(
		pbResourceMetadata.DisplayName,
		/* optional */ true,
	)
	if err != nil {
		return nil, err
	}

	annotations, err := serialisation.FromMappingNodePB(
		pbResourceMetadata.Annotations,
		/* optional */ true,
	)
	if err != nil {
		return nil, err
	}

	custom, err := serialisation.FromMappingNodePB(
		pbResourceMetadata.Custom,
		/* optional */ true,
	)
	if err != nil {
		return nil, err
	}

	return &provider.ResolvedResourceMetadata{
		DisplayName: displayName,
		Annotations: annotations,
		Labels: &schema.StringMap{
			Values: pbResourceMetadata.Labels,
		},
		Custom: custom,
	}, nil
}

func fromPBResourceState(
	pbResourceState *sharedtypesv1.ResourceState,
) (*state.ResourceState, error) {
	if pbResourceState == nil {
		return nil, nil
	}

	specData, err := serialisation.FromMappingNodePB(pbResourceState.SpecData, false /* optional */)
	if err != nil {
		return nil, err
	}

	resourceMetadataState, err := FromPBResourceMetadataState(pbResourceState.Metadata)
	if err != nil {
		return nil, err
	}

	lastDriftDetectedTimestamp := pbutils.IntPtrFromPBWrapper(pbResourceState.LastDriftDetectedTimestamp)

	durations, err := fromPBResourceCompletionDurations(pbResourceState.Durations)
	if err != nil {
		return nil, err
	}

	return &state.ResourceState{
		ResourceID:                 pbResourceState.Id,
		Name:                       pbResourceState.Name,
		Type:                       pbResourceState.Type,
		TemplateName:               pbResourceState.TemplateName,
		InstanceID:                 pbResourceState.InstanceId,
		Status:                     core.ResourceStatus(pbResourceState.Status),
		PreciseStatus:              core.PreciseResourceStatus(pbResourceState.PreciseStatus),
		LastStatusUpdateTimestamp:  int(pbResourceState.LastStatusUpdateTimestamp),
		LastDeployedTimestamp:      int(pbResourceState.LastDeployedTimestamp),
		LastDeployAttemptTimestamp: int(pbResourceState.LastDeployAttemptTimestamp),
		SpecData:                   specData,
		Description:                pbResourceState.Description,
		Metadata:                   resourceMetadataState,
		DependsOnResources:         pbResourceState.DependsOnResources,
		DependsOnChildren:          pbResourceState.DependsOnChildren,
		FailureReasons:             pbResourceState.FailureReasons,
		Drifted:                    pbResourceState.Drifted,
		LastDriftDetectedTimestamp: lastDriftDetectedTimestamp,
		Durations:                  durations,
	}, nil
}

func fromPBResourceCompletionDurations(
	pbDurations *sharedtypesv1.ResourceCompletionDurations,
) (*state.ResourceCompletionDurations, error) {
	if pbDurations == nil {
		return nil, nil
	}

	configCompleteDuration := pbutils.DoublePtrFromPBWrapper(pbDurations.ConfigCompleteDuration)
	return &state.ResourceCompletionDurations{
		ConfigCompleteDuration: configCompleteDuration,
	}, nil
}

// FromPBResourceMetadataState converts a ResourceMetadataState from a protobuf message to a core type
// compatible with the blueprint framework.
func FromPBResourceMetadataState(
	pbResourceMetadataState *sharedtypesv1.ResourceMetadataState,
) (*state.ResourceMetadataState, error) {
	if pbResourceMetadataState == nil {
		return nil, nil
	}

	annotations, err := FromPBMappingNodeMap(pbResourceMetadataState.Annotations)
	if err != nil {
		return nil, err
	}

	custom, err := serialisation.FromMappingNodePB(
		pbResourceMetadataState.Custom,
		/* optional */ true,
	)
	if err != nil {
		return nil, err
	}

	return &state.ResourceMetadataState{
		DisplayName: pbResourceMetadataState.DisplayName,
		Annotations: annotations,
		Labels:      pbResourceMetadataState.Labels,
		Custom:      custom,
	}, nil
}

func fromPBFieldChanges(
	pbFieldChanges []*sharedtypesv1.FieldChange,
) ([]provider.FieldChange, error) {
	fieldChanges := make([]provider.FieldChange, len(pbFieldChanges))

	for i, pbFieldChange := range pbFieldChanges {
		prevValue, err := serialisation.FromMappingNodePB(pbFieldChange.PrevValue, true /* optional */)
		if err != nil {
			return nil, err
		}

		newValue, err := serialisation.FromMappingNodePB(pbFieldChange.NewValue, true /* optional */)
		if err != nil {
			return nil, err
		}

		fieldChanges[i] = provider.FieldChange{
			FieldPath:    pbFieldChange.FieldPath,
			PrevValue:    prevValue,
			NewValue:     newValue,
			MustRecreate: pbFieldChange.MustRecreate,
		}
	}

	return fieldChanges, nil
}

func ptrsFromPBFieldChanges(
	pbFieldChanges []*sharedtypesv1.FieldChange,
) ([]*provider.FieldChange, error) {
	fieldChanges, err := fromPBFieldChanges(pbFieldChanges)
	if err != nil {
		return nil, err
	}

	fieldChangePtrs := make([]*provider.FieldChange, len(fieldChanges))
	for i, fieldChange := range fieldChanges {
		fieldChangePtrs[i] = &fieldChange
	}

	return fieldChangePtrs, nil
}

func fromPBLinkChangesMap(
	pbLinkChangesMap map[string]*sharedtypesv1.LinkChanges,
) (map[string]provider.LinkChanges, error) {
	if pbLinkChangesMap == nil {
		return nil, nil
	}

	linkChangesMap := make(map[string]provider.LinkChanges, len(pbLinkChangesMap))

	for key, pbLinkChanges := range pbLinkChangesMap {
		linkChanges, err := fromPBLinkChanges(pbLinkChanges)
		if err != nil {
			return nil, err
		}

		linkChangesMap[key] = linkChanges
	}

	return linkChangesMap, nil
}

func fromPBLinkChanges(
	pbLinkChanges *sharedtypesv1.LinkChanges,
) (provider.LinkChanges, error) {
	if pbLinkChanges == nil {
		return provider.LinkChanges{}, nil
	}

	modifiedFields, err := ptrsFromPBFieldChanges(pbLinkChanges.ModifiedFields)
	if err != nil {
		return provider.LinkChanges{}, err
	}

	newFields, err := ptrsFromPBFieldChanges(pbLinkChanges.NewFields)
	if err != nil {
		return provider.LinkChanges{}, err
	}

	return provider.LinkChanges{
		ModifiedFields:            modifiedFields,
		NewFields:                 newFields,
		RemovedFields:             pbLinkChanges.RemovedFields,
		UnchangedFields:           pbLinkChanges.UnchangedFields,
		FieldChangesKnownOnDeploy: pbLinkChanges.FieldChangesKnownOnDeploy,
	}, nil
}

// FromPBResourceHasStabilisedRequest converts a ResourceHasStabilisedRequest
// from a protobuf message to a core type compatible with the blueprint framework.
func FromPBResourceHasStabilisedRequest(
	req *sharedtypesv1.ResourceHasStabilisedRequest,
) (*provider.ResourceHasStabilisedInput, error) {
	providerCtx, err := FromPBProviderContext(req.Context)
	if err != nil {
		return nil, err
	}

	resourceSpec, err := serialisation.FromMappingNodePB(req.ResourceSpec, false /* optional */)
	if err != nil {
		return nil, err
	}

	resourceMetadataState, err := FromPBResourceMetadataState(req.ResourceMetadata)
	if err != nil {
		return nil, err
	}

	return &provider.ResourceHasStabilisedInput{
		InstanceID:       req.InstanceId,
		ResourceID:       req.ResourceId,
		ResourceSpec:     resourceSpec,
		ResourceMetadata: resourceMetadataState,
		ProviderContext:  providerCtx,
	}, nil
}

// FromPBProviderContext converts a ProviderContext from a protobuf message to a core type
// compatible with the blueprint framework.
func FromPBProviderContext(pbProviderCtx *sharedtypesv1.ProviderContext) (provider.Context, error) {
	providerConfigVars, err := FromPBScalarMap(pbProviderCtx.ProviderConfigVariables)
	if err != nil {
		return nil, err
	}

	contextVars, err := FromPBScalarMap(pbProviderCtx.ContextVariables)
	if err != nil {
		return nil, err
	}

	return utils.ProviderContextFromVarMaps(providerConfigVars, contextVars), nil
}

// ResourceTypeToString converts a ResourceType to a string.
func ResourceTypeToString(resourceType *sharedtypesv1.ResourceType) string {
	if resourceType == nil {
		return ""
	}

	return resourceType.Type
}

// FromPBDestroyResourceRequest converts a DestroyResourceRequest from a protobuf message to a core type
// compatible with the blueprint framework.
func FromPBDestroyResourceRequest(
	req *sharedtypesv1.DestroyResourceRequest,
) (*provider.ResourceDestroyInput, error) {
	providerCtx, err := FromPBProviderContext(req.Context)
	if err != nil {
		return nil, err
	}

	resourceState, err := fromPBResourceState(req.ResourceState)
	if err != nil {
		return nil, err
	}

	return &provider.ResourceDestroyInput{
		InstanceID:      req.InstanceId,
		ResourceID:      req.ResourceId,
		ResourceState:   resourceState,
		ProviderContext: providerCtx,
	}, nil
}

type functionCallContextInfo struct {
	stack    function.Stack
	params   core.BlueprintParams
	location *source.Meta
}

func fromPBFunctionCallContext(
	reqContext *sharedtypesv1.FunctionCallContext,
) (*functionCallContextInfo, error) {
	if reqContext == nil {
		return nil, errors.New("expected function call context to be non-nil")
	}

	stack, err := fromPBFunctionCallStack(reqContext.CallStack)
	if err != nil {
		return nil, err
	}

	params, err := fromPBBlueprintParams(reqContext.Params)
	if err != nil {
		return nil, err
	}

	location, err := fromPBSourceMeta(reqContext.CurrentLocation)
	if err != nil {
		return nil, err
	}

	return &functionCallContextInfo{
		stack:    stack,
		params:   params,
		location: location,
	}, nil
}

func fromPBFunctionCallStack(
	reqStack []*sharedtypesv1.FunctionCall,
) (function.Stack, error) {
	stack := function.NewStack()
	for _, call := range reqStack {
		location, err := fromPBSourceMeta(call.Location)
		if err != nil {
			return nil, err
		}
		stack.Push(&function.Call{
			FilePath:     call.FilePath,
			FunctionName: call.FunctionName,
			Location:     location,
		})
	}

	return stack, nil
}

func fromPBSourceMeta(
	pbLocation *sharedtypesv1.SourceMeta,
) (*source.Meta, error) {
	if pbLocation == nil {
		return nil, nil
	}

	startPosition, err := fromPBSourcePosition(pbLocation.StartPosition)
	if err != nil {
		return nil, err
	}

	endPosition, err := ptrFromPBSourcePosition(pbLocation.EndPosition)
	if err != nil {
		return nil, err
	}

	return &source.Meta{
		Position:    startPosition,
		EndPosition: endPosition,
	}, nil
}

func fromPBSourcePosition(
	pbPosition *sharedtypesv1.SourcePosition,
) (source.Position, error) {
	if pbPosition == nil {
		return source.Position{
			Line: -1,
		}, nil
	}

	return source.Position{
		Line:   int(pbPosition.Line),
		Column: int(pbPosition.Column),
	}, nil
}

func ptrFromPBSourcePosition(
	pbPosition *sharedtypesv1.SourcePosition,
) (*source.Position, error) {
	if pbPosition == nil {
		return nil, nil
	}

	position, err := fromPBSourcePosition(pbPosition)
	if err != nil {
		return nil, err
	}

	if position.Line == -1 {
		return nil, nil
	}

	return &position, nil
}

func fromPBBlueprintParams(
	reqParams *sharedtypesv1.BlueprintParams,
) (core.BlueprintParams, error) {
	if reqParams == nil {
		return nil, errors.New("expected blueprint params to be non-nil")
	}

	providerConfig, err := FromPBScalarMap(reqParams.ProviderConfigVariables)
	if err != nil {
		return nil, err
	}
	providerConfigExpanded := expandNamespacedConfig(providerConfig)

	transformerConfig, err := FromPBScalarMap(reqParams.TransformerConfigVariables)
	if err != nil {
		return nil, err
	}
	transformerConfigExpanded := expandNamespacedConfig(transformerConfig)

	contextVariables, err := FromPBScalarMap(reqParams.ContextVariables)
	if err != nil {
		return nil, err
	}

	blueprintVariables, err := FromPBScalarMap(reqParams.BlueprintVariables)
	if err != nil {
		return nil, err
	}

	return core.NewDefaultParams(
		providerConfigExpanded,
		transformerConfigExpanded,
		contextVariables,
		blueprintVariables,
	), nil
}

func fromPBFunctionCallArgs(
	reqArgs *sharedtypesv1.FunctionCallArgs,
) ([]any, error) {
	if reqArgs == nil {
		return []any{}, nil
	}

	deserialised, err := pbutils.ConvertPBAnyToInterface(reqArgs.Args)
	if err != nil {
		return nil, err
	}

	args, ok := deserialised.([]any)
	if !ok {
		return nil, fmt.Errorf(
			"expected arguments to be a []any, got %T",
			deserialised,
		)
	}

	return args, nil
}

// FromPBMappingNodeMap converts a map of protobuf MappingNodes to a map of core MappingNodes
// compatible with the blueprint framework.
func FromPBMappingNodeMap(
	pbMap map[string]*schemapb.MappingNode,
) (map[string]*core.MappingNode, error) {
	if pbMap == nil {
		return nil, nil
	}

	coreMap := make(map[string]*core.MappingNode, len(pbMap))
	for key, pbNode := range pbMap {
		coreNode, err := serialisation.FromMappingNodePB(pbNode, true /* optional */)
		if err != nil {
			return nil, err
		}

		coreMap[key] = coreNode
	}

	return coreMap, nil
}

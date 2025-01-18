package container

import (
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
)

func collectExportChanges(
	changes *intermediaryBlueprintChanges,
	resolvedExports map[string]*subengine.ResolveResult,
	currentExportsState map[string]*state.ExportState,
) {
	for exportName, resolvedExport := range resolvedExports {
		exportValue := extractExportValue(exportName, currentExportsState)
		collectExportFieldChanges(
			changes,
			exportName,
			resolvedExport.Resolved,
			exportValue,
			resolvedExport.ResolveOnDeploy,
		)
	}
}

func extractExportValue(exportName string, exports map[string]*state.ExportState) *core.MappingNode {
	exportState, hasExport := exports[exportName]
	if hasExport {
		return exportState.Value
	}

	return nil
}

func collectExportFieldChanges(
	changes *intermediaryBlueprintChanges,
	exportName string,
	newExportValue *core.MappingNode,
	currentStateValue *core.MappingNode,
	fieldsToResolveOnDeploy []string,
) {
	if len(fieldsToResolveOnDeploy) > 0 {
		// If any nested values of the export field value can not be known until deploy time,
		// mark the export field as a field to resolve on deploy.
		changes.ResolveOnDeploy = append(
			changes.ResolveOnDeploy,
			substitutions.RenderFieldPath("exports", exportName),
		)
	}

	if core.IsNilMappingNode(newExportValue) &&
		!core.IsNilMappingNode(currentStateValue) &&
		// Do not mark as removed if some parts of the export field value
		// can not be known until deploy time.
		len(fieldsToResolveOnDeploy) == 0 {
		changes.RemovedExports = append(
			changes.RemovedExports,
			substitutions.RenderFieldPath("exports", exportName),
		)
		return
	}

	if !core.IsNilMappingNode(newExportValue) &&
		core.IsNilMappingNode(currentStateValue) {
		changes.NewExports[exportName] = &provider.FieldChange{
			FieldPath: substitutions.RenderFieldPath("exports", exportName),
			PrevValue: nil,
			NewValue:  newExportValue,
		}
		return
	}

	if !core.MappingNodeEqual(newExportValue, currentStateValue) {
		changes.ExportChanges[exportName] = &provider.FieldChange{
			FieldPath: substitutions.RenderFieldPath("exports", exportName),
			PrevValue: currentStateValue,
			NewValue:  newExportValue,
		}
	} else {
		changes.UnchangedExports = append(
			changes.UnchangedExports,
			substitutions.RenderFieldPath("exports", exportName),
		)
	}
}

func collectMetadataChanges(
	collectIn *MetadataChanges,
	resolveResult *subengine.ResolveInMappingNodeResult,
	currentMetadata map[string]*core.MappingNode,
) {

	if resolveResult.ResolvedMappingNode == nil ||
		!core.IsObjectMappingNode(resolveResult.ResolvedMappingNode) {
		// Blueprint-wide metadata must always be an object/map.
		return
	}

	changes := &provider.Changes{
		NewFields:                 []provider.FieldChange{},
		ModifiedFields:            []provider.FieldChange{},
		RemovedFields:             []string{},
		UnchangedFields:           []string{},
		FieldChangesKnownOnDeploy: resolveResult.ResolveOnDeploy,
	}
	// To reuse the same logic used to collect changes in the `metadata.custom`
	// field of a resource, collect changes in the resource provider.Changes structure
	// and then map the collected changes to the MetadataChanges structure.
	collectMappingNodeChanges(
		changes,
		resolveResult.ResolvedMappingNode,
		&core.MappingNode{
			Fields: currentMetadata,
		},
		&fieldChangeContext{
			fieldsToResolveOnDeploy: resolveResult.ResolveOnDeploy,
			currentPath:             "metadata",
			depth:                   0,
		},
	)

	collectIn.NewFields = changes.NewFields
	collectIn.ModifiedFields = changes.ModifiedFields
	collectIn.RemovedFields = changes.RemovedFields
	collectIn.UnchangedFields = changes.UnchangedFields
}

package container

import (
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
)

func collectExportChanges(
	changes *intermediaryBlueprintChanges,
	resolvedExports map[string]*subengine.ResolveResult,
	currentExportsState map[string]*core.MappingNode,
) {
	for exportName, resolvedExport := range resolvedExports {
		collectExportFieldChanges(
			changes,
			exportName,
			resolvedExport.Resolved,
			currentExportsState[exportName],
			resolvedExport.ResolveOnDeploy,
		)
	}
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

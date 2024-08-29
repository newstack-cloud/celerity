package validation

import (
	"context"
	"fmt"

	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/common/core"
)

// ValidateExport validates an export in a blueprint.
// This ensures that the export type is valid and that the referenced
// field is in the valid format.
// This does not validate that the field of the export can not be resolved,
// as this export validation should be carried out
// before staging changes or deploying a blueprint.
func ValidateExport(
	ctx context.Context,
	exportName string,
	exportSchema *schema.Export,
	exportMap *schema.ExportMap,
) ([]*bpcore.Diagnostic, error) {
	diagnostics := []*bpcore.Diagnostic{}
	err := validateExportType(exportSchema.Type, exportName, exportMap)
	if err != nil {
		return diagnostics, err
	}

	return diagnostics, validateExportFieldFormat(exportSchema.Field, exportName, exportMap)
}

func validateExportType(
	exportType schema.ExportType,
	exportName string,
	exportMap *schema.ExportMap,
) error {
	if !core.SliceContainsComparable(schema.ExportTypes, exportType) {
		return errInvalidExportType(
			exportType,
			exportName,
			getExportSourceMeta(exportMap, exportName),
		)
	}
	return nil
}

func validateExportFieldFormat(exportField, exportName string, exportMap *schema.ExportMap) error {
	if exportField == "" {
		return errEmptyExportField(
			exportName,
			getExportSourceMeta(exportMap, exportName),
		)
	}

	context := fmt.Sprintf("exports.%s", exportName)
	return ValidateReference(exportField, context, ExportCanReference)
}

var (
	// ExportCanReference is a list of objects that can be referenced
	// by an export.
	// In the current version of the specification, resources, data sources,
	// variables, values and child blueprints can be referenced by an export.
	ExportCanReference = []Referenceable{
		ReferenceableResource,
		ReferenceableDataSource,
		ReferenceableVariable,
		ReferenceableValue,
		ReferenceableChild,
	}
)

func getExportSourceMeta(exportMap *schema.ExportMap, varName string) *source.Meta {
	if exportMap == nil {
		return nil
	}

	return exportMap.SourceMeta[varName]
}

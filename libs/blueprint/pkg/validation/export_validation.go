package validation

import (
	"context"
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/common/pkg/core"
)

// ValidateExport validates an export in a blueprint.
// This ensures that the export type is valid and that the referenced
// field is in the valid format.
// This does not validate that the field of the export can not be resolved,
// as this export validation should be carried out
// before staging changes or deploying a blueprint.
func ValidateExport(ctx context.Context, exportName string, exportSchema *schema.Export) error {
	err := validateExportType(exportSchema.Type, exportName)
	if err != nil {
		return err
	}

	return validateExportFieldFormat(exportSchema.Field, exportName)
}

func validateExportType(exportType schema.ExportType, exportName string) error {
	if !core.SliceContainsComparable(schema.ExportTypes, exportType) {
		return errInvalidExportType(exportType, exportName)
	}
	return nil
}

func validateExportFieldFormat(exportField, exportName string) error {
	if exportField == "" {
		return errEmptyExportField(exportName)
	}

	context := fmt.Sprintf("exports.%s", exportName)
	return ValidateReference(exportField, context, ExportCanReference)
}

var (
	// ExportCanReference is a list of objects that can be referenced
	// by an export.
	ExportCanReference = []Referenceable{
		ReferenceableVariable,
		ReferenceableChild,
		ReferenceableDataSource,
		ReferenceableResource,
	}
)

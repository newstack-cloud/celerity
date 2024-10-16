package core

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/core"
)

// BuildEngine provides an interface for a service that provides
// all the capabilities needed to validate, build, and deploy a
// Celerity project.
type BuildEngine interface {
	// Validates a Celerity project or blueprint.
	// For a project, this should validate the project structure,
	// the blueprint file and other configuration
	// depending on the programming language along with the source code.
	// When blueprintOnly is set to true, this should only validate the blueprint.
	Validate(ctx context.Context, params *ValidateParams) (*ValidateResults, error)
	// Validates a Celerity project or blueprint,
	// This is a streaming version of the Validate method,
	// a stream of validation results are expected to be sent to the out channel.
	// If an error occurs during validation, it should be sent to the err channel.
	// A nil value should be sent to the out channel when validation is complete.
	ValidateStream(ctx context.Context, params *ValidateParams, out chan<- *ValidateResult, errChan chan<- error) error
}

// ValidateParams is a struct that contains all the parameters
// needed to validate a Celerity project or blueprint.
type ValidateParams struct {
	// The scheme for the location of the source code, config and blueprint
	// files to validate.
	// Examples would be "file://" for the local file system, "s3://" for
	// an S3 bucket.
	// This should default to "file://", see the specific implementation for details.
	// Implementations can support multiple sources but all implementations
	// must support the file:// scheme.
	FileSourceScheme *string `json:"fileSourceScheme,omitempty"`
	// The absolute path to the directory of the project or blueprint to validate.
	// This should default to the current working directory when the "file://" scheme
	// is used for the OS file system.
	// See the specific implementation for details.
	Directory *string `json:"directory,omitempty"`
	// The name of the blueprint file to validate.
	// This should default to "app.blueprint.yaml", see the specific
	// implementation for details.
	BlueprintFile *string `json:"blueprintFile,omitempty"`
	// When set to true, this should only validate the blueprint file.
	// This should default to false, see the specific implementation for details.
	BlueprintOnly *bool `json:"blueprintOnly,omitempty"`
}

// ValidateResults contains the full results of validation for a project
// or blueprint.
type ValidateResults struct {
	GroupedResults []*GroupedValidateResults `json:"groupedResults"`
}

// GroupedValidateResults contains a group of validation results
// grouped by category and file path.
type GroupedValidateResults struct {
	Category ValidationCategory `json:"category"`
	// The path to the file that the diagnostic is related to.
	// This will be an absolute path on the machine running the validation.
	// This will be nil if the diagnostic is not related to a file.
	FilePath    *string            `json:"filePath,omitempty"`
	Diagnostics []*core.Diagnostic `json:"diagnostics"`
}

// ValidateResult is a struct that contains a diagnostic and the context
// of the validation result (file path, source code/blueprint/configuration, etc.).
type ValidateResult struct {
	Category ValidationCategory `json:"category"`
	// The path to the file that the diagnostic is related to.
	// This will be an absolute path on the machine running the validation.
	// This will be nil if the diagnostic is not related to a file.
	FilePath   *string          `json:"filePath,omitempty"`
	Diagnostic *core.Diagnostic `json:"diagnostic"`
}

// ValidationCategory is an enum that represents the category of the validation result.
type ValidationCategory string

const (
	// ValidationCategoryBluperint represents a validation result that is related to the blueprint.
	ValidationCategoryBlueprint ValidationCategory = "blueprint"
	// ValidationCategorySource represents a validation result that is related to the source code.
	ValidationCategorySource ValidationCategory = "source"
	// ValidationCategoryConfig represents a validation result that is related to additional
	// configuration that is not part of the blueprint or source code.
	ValidationCategoryConfig ValidationCategory = "config"
)

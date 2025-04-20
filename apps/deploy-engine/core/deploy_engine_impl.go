package core

import (
	"context"
	"os"
	"path"

	"github.com/spf13/afero"
	"github.com/two-hundred/celerity/apps/deploy-engine/utils"
	"github.com/two-hundred/celerity/libs/blueprint/container"
	"github.com/two-hundred/celerity/libs/blueprint/core"
)

const (
	// SchemeFileSystemOS is the scheme for the OS file system.
	SchemeFileSystemOS = "file://"
)

const (
	// TODO: work only with absolute paths for validation,
	// the deploy engine doesn't work relative to the current working directory
	// as can be called remotely.
	// DefaultBlueprintFile is the default blueprint file name.
	DefaultBlueprintFile = "app.blueprint.yaml"
)

type deployEngineImpl struct {
	validateLoader container.Loader
	deployLoader   container.Loader
	fileSystems    map[string]afero.Fs
	logger         core.Logger
}

// DefaultDeployEngineOption is an option for the default DeployEngine implementation.
type DefaultDeployEngineOption func(*deployEngineImpl)

// WithFileSystems sets the file systems to use for the given scheme
// in the default DeployEngine implementation.
// The scheme should contain the trailing "://" (e.g. "file://").
func WithFileSystems(scheme string, fileSystem afero.Fs) DefaultDeployEngineOption {
	return func(b *deployEngineImpl) {
		b.fileSystems[scheme] = fileSystem
	}
}

// NewDefaultDeployEngine creates a new instance of the default DeployEngine implementation.
func NewDefaultDeployEngine(
	validateLoader container.Loader,
	deployLoader container.Loader,
	logger core.Logger,
	opts ...DefaultDeployEngineOption,
) DeployEngine {
	engine := &deployEngineImpl{
		validateLoader: validateLoader,
		deployLoader:   deployLoader,
		logger:         logger,
		fileSystems:    make(map[string]afero.Fs),
	}

	for _, opt := range opts {
		opt(engine)
	}

	_, hasOSFS := engine.fileSystems[SchemeFileSystemOS]
	if !hasOSFS {
		engine.fileSystems[SchemeFileSystemOS] = afero.NewOsFs()
	}

	return engine
}

// Validate a Celerity project or blueprint.
// For a project, this validates the project structure,
// the blueprint file and other configuration
// depending on the programming language along with the source code.
// When blueprintOnly is set to true, this only validate the blueprint.
func (b *deployEngineImpl) Validate(ctx context.Context, params *ValidateParams) (*ValidateResults, error) {
	fs, err := b.getFileSystem(params.FileSourceScheme)
	if err != nil {
		return nil, err
	}

	filePath, diagnostics, err := b.validateBlueprint(params, fs)
	if err != nil {
		return nil, err
	}

	results := &ValidateResults{
		GroupedResults: []*GroupedValidateResults{
			{
				Category:    ValidationCategoryBlueprint,
				FilePath:    &filePath,
				Diagnostics: diagnostics,
			},
		},
	}

	if params.BlueprintOnly != nil && *params.BlueprintOnly {
		return results, nil
	}

	// todo: validate project structure, source code, and other configuration

	return results, nil
}

func (b *deployEngineImpl) validateBlueprint(params *ValidateParams, fs afero.Fs) (string, []*core.Diagnostic, error) {
	filePath, err := b.determineFilePath(
		params.FileSourceScheme,
		params.Directory,
		params.BlueprintFile,
	)
	if err != nil {
		return filePath, nil, err
	}

	blueprintBytes, err := afero.ReadFile(fs, filePath)
	if err != nil {
		return filePath, nil, err
	}

	blueprintFormat, err := utils.BlueprintFormatFromExtension(filePath)
	if err != nil {
		return filePath, nil, err
	}

	blueprintParams := core.NewDefaultParams(
		map[string]map[string]*core.ScalarValue{},
		map[string]map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
	)
	validationResult, err := b.validateLoader.ValidateString(
		context.TODO(),
		string(blueprintBytes),
		blueprintFormat,
		blueprintParams,
	)
	// Validation errors are converted to diagnostics to provide a consistent
	// experience for the user, the only errors that should be returned are failures
	// outside of the validation process.
	errDiagnostics := utils.DiagnosticsFromBlueprintValidationError(err, b.logger)

	return filePath, append(validationResult.Diagnostics, errDiagnostics...), nil
}

func (b *deployEngineImpl) validateBlueprintAsync(
	ctx context.Context,
	params *ValidateParams,
	fs afero.Fs,
	out chan<- *ValidateResult,
	errChan chan<- error,
) {
	b.logger.Info("Blueprint validation started")

	filePath, diagnostics, err := b.validateBlueprint(params, fs)
	if err != nil {
		errChan <- err
		return
	}

	b.logger.Info("Blueprint validation complete", core.StringLogField("file", filePath))

	for _, diagnostic := range diagnostics {
		out <- &ValidateResult{
			Category:   ValidationCategoryBlueprint,
			FilePath:   &filePath,
			Diagnostic: diagnostic,
		}
	}

	out <- nil
}

// ValidateStream validates a Celerity project or blueprint.
// This is a streaming version of the Validate method,
// a stream of validation results are sent to the out channel.
// If an error occurs during validation, it is sent to the err channel.
// A nil value is sent to the out channel when validation is complete.
func (b *deployEngineImpl) ValidateStream(
	ctx context.Context,
	params *ValidateParams,
	out chan<- *ValidateResult,
	errChan chan<- error,
) error {
	b.logger.Info("Deriving file system for validation")
	fs, err := b.getFileSystem(params.FileSourceScheme)
	if err != nil {
		return err
	}

	go b.validateBlueprintAsync(ctx, params, fs, out, errChan)

	return nil
}

func (b *deployEngineImpl) getFileSystem(scheme *string) (afero.Fs, error) {
	if scheme == nil {
		return b.fileSystems[SchemeFileSystemOS], nil
	}

	fs, ok := b.fileSystems[*scheme]
	if !ok {
		return nil, ErrFileSystemNotFound
	}

	return fs, nil
}

func (b *deployEngineImpl) determineFilePath(
	scheme *string,
	directory *string,
	blueprintFile *string,
) (string, error) {
	finalScheme := SchemeFileSystemOS
	if scheme != nil {
		finalScheme = *scheme
	}

	finalDir := ""
	if directory == nil && finalScheme != SchemeFileSystemOS {
		var err error
		finalDir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	} else if directory != nil {
		finalDir = *directory
	}

	finalBlueprintFile := DefaultBlueprintFile
	if blueprintFile != nil {
		finalBlueprintFile = *blueprintFile
	}

	return path.Join(finalDir, finalBlueprintFile), nil
}

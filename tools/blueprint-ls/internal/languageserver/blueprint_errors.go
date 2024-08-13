package languageserver

import (
	"reflect"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/errors"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
	"go.uber.org/zap"
)

func blueprintErrorToDiagnostics(err error, logger *zap.Logger) []lsp.Diagnostic {
	diagnostics := []lsp.Diagnostic{}

	logger.Debug("blueprintErrorToDiagnostics, err type", zap.String("type", reflect.TypeOf(err).String()))
	loadErr, isLoadErr := err.(*errors.LoadError)
	if isLoadErr {
		collectLoadErrors(loadErr, &diagnostics, nil, logger)
		return diagnostics
	}

	return getGeneralErrorDiagnostics(err)
}

func getGeneralErrorDiagnostics(err error) []lsp.Diagnostic {
	severity := lsp.DiagnosticSeverityError
	return []lsp.Diagnostic{
		{
			Range: lsp.Range{
				Start: lsp.Position{
					Line:      0,
					Character: 0,
				},
				End: lsp.Position{
					Line:      1,
					Character: 0,
				},
			},
			Severity: &severity,
			Message:  err.Error(),
		},
	}
}

func collectLoadErrors(
	err *errors.LoadError,
	diagnostics *[]lsp.Diagnostic,
	parentLoadErr *errors.LoadError,
	logger *zap.Logger,
) {
	logger.Debug("load error", zap.String("error", err.Error()), zap.Int("child error count", len(err.ChildErrors)))
	for _, childErr := range err.ChildErrors {
		logger.Debug("child error type", zap.String("type", reflect.TypeOf(childErr).String()))
		childLoadErr, isLoadErr := childErr.(*errors.LoadError)
		if isLoadErr {
			logger.Debug("child load error", zap.String("error", childLoadErr.Error()))
			collectLoadErrors(childLoadErr, diagnostics, err, logger)
		}

		childParseErrs, isParseErrs := childErr.(*substitutions.ParseErrors)
		if isParseErrs {
			collectParseErrors(childParseErrs, diagnostics, err)
		}

		childParseErr, isParseErr := childErr.(*substitutions.ParseError)
		if isParseErr {
			collectParseError(childParseErr, diagnostics)
		}

		childCoreErr, isCoreErr := childErr.(*core.Error)
		if isCoreErr {
			collectCoreError(childCoreErr, diagnostics, err)
		}

		childLexErrors, isLexErrs := childErr.(*substitutions.LexErrors)
		if isLexErrs {
			collectLexErrors(childLexErrors, diagnostics, err)
		}

		childLexError, isLexErr := childErr.(*substitutions.LexError)
		if isLexErr {
			collectLexError(childLexError, diagnostics)
		}

		if !isLoadErr && !isParseErrs && !isParseErr && !isCoreErr && !isLexErrs && !isLexErr {
			collectGeneralError(childErr, diagnostics, err)
		}
	}

	if len(err.ChildErrors) == 0 {
		logger.Debug("adding diagnostics for error", zap.String("error", err.Error()))
		line, col := positionFromLoadError(err, parentLoadErr)
		severity := lsp.DiagnosticSeverityError
		*diagnostics = append(*diagnostics, lsp.Diagnostic{
			Range: lsp.Range{
				Start: lsp.Position{
					Line:      line,
					Character: col,
				},
				End: lsp.Position{
					Line:      line + 1,
					Character: 0,
				},
			},
			Severity: &severity,
			Message:  err.Error(),
		})
	}
}

func positionFromLoadError(
	err *errors.LoadError,
	parentLoadErr *errors.LoadError,
) (lsp.UInteger, lsp.UInteger) {
	errMissingLines := err.Line == nil && err.Column == nil
	parentMissingLines := parentLoadErr == nil || parentLoadErr.Line == nil && parentLoadErr.Column == nil

	if errMissingLines && parentMissingLines {
		return lsp.UInteger(0), lsp.UInteger(0)
	}

	if errMissingLines {
		// LSP offsets are 0-based, the blueprint package uses 1-based offsets.
		return lsp.UInteger(*parentLoadErr.Line - 1), lsp.UInteger(*parentLoadErr.Column - 1)
	}

	// LSP offsets are 0-based, the blueprint package uses 1-based offsets.
	return lsp.UInteger(*err.Line - 1), lsp.UInteger(*err.Column - 1)
}

func collectParseErrors(
	errs *substitutions.ParseErrors,
	diagnostics *[]lsp.Diagnostic,
	parentLoadErr *errors.LoadError,
) {
	for _, childErr := range errs.ChildErrors {
		childParseErr, isParseError := childErr.(*substitutions.ParseError)
		if isParseError {
			collectParseError(childParseErr, diagnostics)
		}
	}

	if len(errs.ChildErrors) == 0 {
		line, col := positionFromParentLoadError(parentLoadErr)
		severity := lsp.DiagnosticSeverityError
		*diagnostics = append(*diagnostics, lsp.Diagnostic{
			Range: lsp.Range{
				Start: lsp.Position{
					Line:      line,
					Character: col,
				},
				End: lsp.Position{
					Line:      line + 1,
					Character: 0,
				},
			},
			Severity: &severity,
			Message:  errs.Error(),
		})
	}
}

func collectParseError(
	err *substitutions.ParseError,
	diagnostics *[]lsp.Diagnostic,
) {
	line, col := positionFromParseError(err)
	severity := lsp.DiagnosticSeverityError
	*diagnostics = append(*diagnostics, lsp.Diagnostic{
		Range: lsp.Range{
			Start: lsp.Position{
				Line:      line,
				Character: col,
			},
			End: lsp.Position{
				Line:      line + 1,
				Character: 0,
			},
		},
		Severity: &severity,
		Message:  err.Error(),
	})
}

func positionFromParseError(
	err *substitutions.ParseError,
) (lsp.UInteger, lsp.UInteger) {
	col := lsp.UInteger(0)
	if err.ColumnAccuracy == substitutions.ColumnAccuracyExact {
		// LSP offsets are 0-based, the blueprint package uses 1-based offsets.
		col = lsp.UInteger(err.Column - 1)
	}

	// LSP offsets are 0-based, the blueprint package uses 1-based offsets.
	return lsp.UInteger(err.Line - 1), col
}

func positionFromParentLoadError(
	parentLoadErr *errors.LoadError,
) (lsp.UInteger, lsp.UInteger) {
	parentMissingLines := parentLoadErr == nil || parentLoadErr.Line == nil && parentLoadErr.Column == nil

	if parentMissingLines {
		return lsp.UInteger(0), lsp.UInteger(0)
	}

	// LSP offsets are 0-based, the blueprint package uses 1-based offsets.
	return lsp.UInteger(*parentLoadErr.Line - 1), lsp.UInteger(*parentLoadErr.Column - 1)
}

func collectCoreError(
	err *core.Error,
	diagnostics *[]lsp.Diagnostic,
	parentLoadErr *errors.LoadError,
) {
	line, col := positionFromCoreError(err, parentLoadErr)
	severity := lsp.DiagnosticSeverityError
	*diagnostics = append(*diagnostics, lsp.Diagnostic{
		Range: lsp.Range{
			Start: lsp.Position{
				Line:      line,
				Character: col,
			},
			End: lsp.Position{
				Line:      line + 1,
				Character: 0,
			},
		},
		Severity: &severity,
		Message:  err.Error(),
	})
}

func positionFromCoreError(
	err *core.Error,
	parentLoadErr *errors.LoadError,
) (lsp.UInteger, lsp.UInteger) {
	errMissingLines := err.SourceLine == nil && err.SourceColumn == nil
	parentMissingLines := parentLoadErr == nil || parentLoadErr.Line == nil && parentLoadErr.Column == nil

	if errMissingLines && parentMissingLines {
		return lsp.UInteger(0), lsp.UInteger(0)
	}

	if errMissingLines {
		// LSP offsets are 0-based, the blueprint package uses 1-based offsets.
		return lsp.UInteger(*parentLoadErr.Line - 1), lsp.UInteger(*parentLoadErr.Column - 1)
	}

	// LSP offsets are 0-based, the blueprint package uses 1-based offsets.
	return lsp.UInteger(*err.SourceLine - 1), lsp.UInteger(*err.SourceColumn - 1)
}

func collectLexErrors(
	errs *substitutions.LexErrors,
	diagnostics *[]lsp.Diagnostic,
	parentLoadErr *errors.LoadError,
) {
	for _, childErr := range errs.ChildErrors {
		childLexErr, isLexError := childErr.(*substitutions.LexError)
		if isLexError {
			collectLexError(childLexErr, diagnostics)
		}
	}

	if len(errs.ChildErrors) == 0 {
		line, col := positionFromParentLoadError(parentLoadErr)
		severity := lsp.DiagnosticSeverityError
		*diagnostics = append(*diagnostics, lsp.Diagnostic{
			Range: lsp.Range{
				Start: lsp.Position{
					Line:      line,
					Character: col,
				},
				End: lsp.Position{
					Line:      line + 1,
					Character: 0,
				},
			},
			Severity: &severity,
			Message:  errs.Error(),
		})
	}
}

func collectLexError(
	err *substitutions.LexError,
	diagnostics *[]lsp.Diagnostic,
) {
	line, col := positionFromLexError(err)
	severity := lsp.DiagnosticSeverityError
	*diagnostics = append(*diagnostics, lsp.Diagnostic{
		Range: lsp.Range{
			Start: lsp.Position{
				Line:      line,
				Character: col,
			},
			End: lsp.Position{
				Line:      line + 1,
				Character: 0,
			},
		},
		Severity: &severity,
		Message:  err.Error(),
	})
}

func positionFromLexError(
	err *substitutions.LexError,
) (lsp.UInteger, lsp.UInteger) {
	col := lsp.UInteger(0)
	if err.ColumnAccuracy == substitutions.ColumnAccuracyExact {
		// LSP offsets are 0-based, the blueprint package uses 1-based offsets.
		col = lsp.UInteger(err.Column - 1)
	}

	// LSP offsets are 0-based, the blueprint package uses 1-based offsets.
	return lsp.UInteger(err.Line - 1), col
}

func collectGeneralError(err error, diagnostics *[]lsp.Diagnostic, parentLoadError *errors.LoadError) {
	severity := lsp.DiagnosticSeverityError
	line, col := positionFromParentLoadError(parentLoadError)
	*diagnostics = append(*diagnostics, lsp.Diagnostic{
		Range: lsp.Range{
			Start: lsp.Position{
				Line:      line,
				Character: col,
			},
			End: lsp.Position{
				Line:      line + 1,
				Character: 0,
			},
		},
		Severity: &severity,
		Message:  err.Error(),
	})
}

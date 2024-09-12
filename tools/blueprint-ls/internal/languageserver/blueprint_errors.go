package languageserver

import (
	"reflect"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/errors"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"github.com/two-hundred/celerity/tools/blueprint-ls/internal/blueprint"
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
	"go.uber.org/zap"
)

func blueprintErrorToDiagnostics(
	err error,
	docURI lsp.URI,
	state *State,
	logger *zap.Logger,
) []lsp.Diagnostic {
	diagnostics := []lsp.Diagnostic{}

	logger.Debug("blueprintErrorToDiagnostics, err type", zap.String("type", reflect.TypeOf(err).String()))
	loadErr, isLoadErr := err.(*errors.LoadError)
	if isLoadErr {
		collectLoadErrors(loadErr, &diagnostics, nil, docURI, state, logger)
		return diagnostics
	}

	schemaErr, isSchemaErr := err.(*schema.Error)
	if isSchemaErr {
		collectSchemaError(schemaErr, &diagnostics, docURI, state)
		return diagnostics
	}

	_, isRunErr := err.(*errors.RunError)
	if isRunErr {
		// Skip capturing run errors during validation,
		// they are useful at runtime but may appear during validation
		// in loading provider and transformer plugins.
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
	docURI lsp.URI,
	state *State,
	logger *zap.Logger,
) {
	logger.Debug("load error", zap.String("error", err.Error()), zap.Int("child error count", len(err.ChildErrors)))
	for _, childErr := range err.ChildErrors {
		logger.Debug("child error type", zap.String("type", reflect.TypeOf(childErr).String()))
		childLoadErr, isLoadErr := childErr.(*errors.LoadError)
		if isLoadErr {
			logger.Debug("child load error", zap.String("error", childLoadErr.Error()))
			collectLoadErrors(childLoadErr, diagnostics, err, docURI, state, logger)
		}

		childSchemaErr, isSchemaErr := childErr.(*schema.Error)
		if isSchemaErr {
			logger.Debug("child schema error", zap.String("error", childSchemaErr.Error()))
			collectSchemaError(childSchemaErr, diagnostics, docURI, state)
		}

		childParseErrs, isParseErrs := childErr.(*substitutions.ParseErrors)
		if isParseErrs {
			collectParseErrors(childParseErrs, diagnostics, err, docURI, state)
		}

		childParseErr, isParseErr := childErr.(*substitutions.ParseError)
		if isParseErr {
			collectParseError(childParseErr, diagnostics, docURI, state)
		}

		childCoreErr, isCoreErr := childErr.(*core.Error)
		if isCoreErr {
			collectCoreError(childCoreErr, diagnostics, err, docURI, state)
		}

		childLexErrors, isLexErrs := childErr.(*substitutions.LexErrors)
		if isLexErrs {
			collectLexErrors(childLexErrors, diagnostics, err, docURI, state)
		}

		childLexError, isLexErr := childErr.(*substitutions.LexError)
		if isLexErr {
			collectLexError(childLexError, diagnostics, docURI, state)
		}

		_, isRunErr := childErr.(*errors.RunError)

		// Skip capturing run errors during validation,
		// they are useful at runtime but may appear during validation
		// in loading provider and transformer plugins.
		if !isRunErr && !isLoadErr && !isParseErrs && !isParseErr &&
			!isCoreErr && !isLexErrs && !isLexErr {
			collectGeneralError(childErr, diagnostics, err, docURI, state)
		}
	}

	if len(err.ChildErrors) == 0 {
		logger.Debug("adding diagnostics for error", zap.String("error", err.Error()))
		severity := lsp.DiagnosticSeverityError
		*diagnostics = append(*diagnostics, lsp.Diagnostic{
			Range: rangeFromBlueprintErrorLocation(
				&blueprintErrorLocationLoadErr{err},
				&blueprintErrorLocationLoadErr{parentLoadErr},
				docURI,
				state,
			),
			Severity: &severity,
			Message:  err.Error(),
		})
	}
}

func rangeFromBlueprintErrorLocation(
	location blueprintErrorLocation,
	parentLocation blueprintErrorLocation,
	docURI lsp.URI,
	state *State,
) lsp.Range {
	errMissingLocation := location.Line() == nil && location.Column() == nil
	parentMissingLocation := parentLocation == nil ||
		parentLocation.Line() == nil && parentLocation.Column() == nil

	if errMissingLocation && parentMissingLocation {
		return lsp.Range{
			Start: lsp.Position{
				Line:      0,
				Character: 0,
			},
			End: lsp.Position{
				Line:      1,
				Character: 0,
			},
		}
	}

	if errMissingLocation {
		return rangeFromBlueprintErrorLocation(
			parentLocation,
			nil,
			docURI,
			state,
		)
	}

	startPos := getStartErrorLocation(location)

	// Get accurate end position for the element that the error is associated with.
	node := state.GetDocumentPositionMapSmallestNode(
		docURI,
		blueprint.PositionKey(startPos),
	)

	if node == nil || node.Range == nil || node.Range.Start == nil {
		// LSP offsets are 0-based, the blueprint package uses 1-based offsets.
		start := lsp.Position{
			Line:      lsp.UInteger(startPos.Line - 1),
			Character: lsp.UInteger(startPos.Column - 1),
		}
		return lsp.Range{
			Start: start,
			End: lsp.Position{
				Line:      start.Line + 1,
				Character: 0,
			},
		}
	}

	endPos := node.Range.End
	if endPos == nil {
		endPos = &source.Meta{
			Line:   node.Range.Start.Line + 1,
			Column: 0,
		}
	}

	return lsp.Range{
		Start: lsp.Position{
			Line:      lsp.UInteger(node.Range.Start.Line - 1),
			Character: lsp.UInteger(node.Range.Start.Column - 1),
		},
		End: lsp.Position{
			Line:      lsp.UInteger(endPos.Line - 1),
			Character: lsp.UInteger(endPos.Column - 1),
		},
	}
}

func getStartErrorLocation(location blueprintErrorLocation) *source.Meta {
	line := location.Line()
	if line == nil {
		firstLine := 1
		line = &firstLine
	}

	col := 1
	colAccuracy := location.ColumnAccuracy()
	if location.UseColumnAccuracy() && colAccuracy != nil {
		if *colAccuracy == substitutions.ColumnAccuracyExact {
			colPtr := location.Column()
			if colPtr != nil {
				col = *colPtr
			}
		}
	} else if !location.UseColumnAccuracy() {
		colPtr := location.Column()
		if colPtr != nil {
			col = *colPtr
		}
	}

	return &source.Meta{
		Line:   *line,
		Column: col,
	}
}

func collectParseErrors(
	errs *substitutions.ParseErrors,
	diagnostics *[]lsp.Diagnostic,
	parentLoadErr *errors.LoadError,
	docURI lsp.URI,
	state *State,
) {
	for _, childErr := range errs.ChildErrors {
		childParseErr, isParseError := childErr.(*substitutions.ParseError)
		if isParseError {
			collectParseError(childParseErr, diagnostics, docURI, state)
		}
	}

	if len(errs.ChildErrors) == 0 {
		severity := lsp.DiagnosticSeverityError
		*diagnostics = append(*diagnostics, lsp.Diagnostic{
			Range: rangeFromBlueprintErrorLocation(
				&blueprintErrorLocationLoadErr{parentLoadErr},
				nil,
				docURI,
				state,
			),
			Severity: &severity,
			Message:  errs.Error(),
		})
	}
}

func collectParseError(
	err *substitutions.ParseError,
	diagnostics *[]lsp.Diagnostic,
	docURI lsp.URI,
	state *State,
) {
	severity := lsp.DiagnosticSeverityError
	*diagnostics = append(*diagnostics, lsp.Diagnostic{
		Range: rangeFromBlueprintErrorLocation(
			&blueprintErrorLocationParseErr{err},
			nil,
			docURI,
			state,
		),
		Severity: &severity,
		Message:  err.Error(),
	})
}

func collectCoreError(
	err *core.Error,
	diagnostics *[]lsp.Diagnostic,
	parentLoadErr *errors.LoadError,
	docURI lsp.URI,
	state *State,
) {
	severity := lsp.DiagnosticSeverityError
	*diagnostics = append(*diagnostics, lsp.Diagnostic{
		Range: rangeFromBlueprintErrorLocation(
			&blueprintErrorLocationCoreErr{err},
			&blueprintErrorLocationLoadErr{parentLoadErr},
			docURI,
			state,
		),
		Severity: &severity,
		Message:  err.Error(),
	})
}

func collectLexErrors(
	errs *substitutions.LexErrors,
	diagnostics *[]lsp.Diagnostic,
	parentLoadErr *errors.LoadError,
	docURI lsp.URI,
	state *State,
) {
	for _, childErr := range errs.ChildErrors {
		childLexErr, isLexError := childErr.(*substitutions.LexError)
		if isLexError {
			collectLexError(childLexErr, diagnostics, docURI, state)
		}
	}

	if len(errs.ChildErrors) == 0 {
		severity := lsp.DiagnosticSeverityError
		*diagnostics = append(*diagnostics, lsp.Diagnostic{
			Range: rangeFromBlueprintErrorLocation(
				&blueprintErrorLocationLoadErr{parentLoadErr},
				nil,
				docURI,
				state,
			),
			Severity: &severity,
			Message:  errs.Error(),
		})
	}
}

func collectLexError(
	err *substitutions.LexError,
	diagnostics *[]lsp.Diagnostic,
	docURI lsp.URI,
	state *State,
) {
	severity := lsp.DiagnosticSeverityError
	*diagnostics = append(*diagnostics, lsp.Diagnostic{
		Range: rangeFromBlueprintErrorLocation(
			&blueprintErrorLocationLexErr{err},
			nil,
			docURI,
			state,
		),
		Severity: &severity,
		Message:  err.Error(),
	})
}

func collectSchemaError(
	err *schema.Error,
	diagnostics *[]lsp.Diagnostic,
	docURI lsp.URI,
	state *State,
) {
	severity := lsp.DiagnosticSeverityError
	*diagnostics = append(*diagnostics, lsp.Diagnostic{
		Range: rangeFromBlueprintErrorLocation(
			&blueprintErrorLocationSchemaErr{err},
			nil,
			docURI,
			state,
		),
		Severity: &severity,
		Message:  err.Error(),
	})
}

func collectGeneralError(
	err error,
	diagnostics *[]lsp.Diagnostic,
	parentLoadError *errors.LoadError,
	docURI lsp.URI,
	state *State,
) {
	severity := lsp.DiagnosticSeverityError
	*diagnostics = append(*diagnostics, lsp.Diagnostic{
		Range: rangeFromBlueprintErrorLocation(
			&blueprintErrorLocationLoadErr{parentLoadError},
			nil,
			docURI,
			state,
		),
		Severity: &severity,
		Message:  err.Error(),
	})
}

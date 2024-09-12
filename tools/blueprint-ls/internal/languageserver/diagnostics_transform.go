package languageserver

import (
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/celerity/libs/blueprint/substitutions"
	"github.com/two-hundred/celerity/tools/blueprint-ls/internal/blueprint"
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
)

func lspDiagnosticsFromBlueprintDiagnostics(
	docURI lsp.URI,
	bpDiagnostics []*core.Diagnostic,
	state *State,
) []lsp.Diagnostic {
	lspDiagnostics := []lsp.Diagnostic{}

	for _, bpDiagnostic := range bpDiagnostics {
		severity := lsp.DiagnosticSeverityInformation
		if bpDiagnostic.Level == core.DiagnosticLevelWarning {
			severity = lsp.DiagnosticSeverityWarning
		} else if bpDiagnostic.Level == core.DiagnosticLevelError {
			severity = lsp.DiagnosticSeverityError
		}

		node := state.GetDocumentPositionMapSmallestNode(
			docURI,
			blueprint.PositionKey(bpDiagnostic.Range.Start),
		)

		lspDiagnostics = append(lspDiagnostics, lsp.Diagnostic{
			Severity: &severity,
			Message:  bpDiagnostic.Message,
			Range: lspDiagnosticRangeFromBlueprintDiagnosticRange(
				createFinalDiagnosticRange(node, bpDiagnostic.Range),
			),
		})
	}

	return lspDiagnostics
}

func lspDiagnosticRangeFromBlueprintDiagnosticRange(bpRange *core.DiagnosticRange) lsp.Range {
	if bpRange == nil {
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

	start := lspPositionFromSourceMeta(bpRange.Start, nil, bpRange.ColumnAccuracy)
	end := lspPositionFromSourceMeta(bpRange.End, &start, bpRange.ColumnAccuracy)

	return lsp.Range{
		Start: start,
		End:   end,
	}
}

func lspPositionFromSourceMeta(
	sourceMeta *source.Meta,
	startPos *lsp.Position,
	columnAccuracy *substitutions.ColumnAccuracy,
) lsp.Position {
	if sourceMeta == nil && startPos == nil {
		return lsp.Position{
			Line:      0,
			Character: 0,
		}
	}

	if sourceMeta == nil && startPos != nil {
		return lsp.Position{
			Line:      startPos.Line + 1,
			Character: 0,
		}
	}

	// When columnAccuracy is nil, it is assumed this diagnostic is not in a substitution
	// context.
	if columnAccuracy != nil && *columnAccuracy == substitutions.ColumnAccuracyApproximate {
		return lsp.Position{
			// LSP offsets are 0-based, the blueprint package uses 1-based offsets.
			Line:      lsp.UInteger(sourceMeta.Line - 1),
			Character: lsp.UInteger(0),
		}
	}

	return lsp.Position{
		// LSP offsets are 0-based, the blueprint package uses 1-based offsets.
		Line:      lsp.UInteger(sourceMeta.Line - 1),
		Character: lsp.UInteger(sourceMeta.Column - 1),
	}
}

func createFinalDiagnosticRange(node *schema.TreeNode, bpRange *core.DiagnosticRange) *core.DiagnosticRange {
	if node == nil || node.Range == nil || bpRange == nil {
		return bpRange
	}

	return &core.DiagnosticRange{
		Start: node.Range.Start,
		End:   node.Range.End,
		// Retain the column accuracy from the original diagnostic.
		ColumnAccuracy: bpRange.ColumnAccuracy,
	}
}

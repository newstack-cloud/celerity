package languageserver

import (
	"github.com/two-hundred/celerity/libs/blueprint/source"
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
)

func containsLSPPoint(bpRange *source.Range, lspPos lsp.Position) bool {
	bpPos := &source.Meta{
		Line:   int(lspPos.Line + 1),
		Column: int(lspPos.Character + 1),
	}

	if bpRange.End == nil {
		return bpPos.Line > bpRange.Start.Line ||
			(bpPos.Line == bpRange.Start.Line && bpPos.Column >= bpRange.Start.Column)
	}

	// Check in range on start line.
	if bpPos.Line == bpRange.Start.Line {
		return bpPos.Column >= bpRange.Start.Column
	}

	// Check in range on end line.
	if bpPos.Line == bpRange.End.Line {
		return bpPos.Column <= bpRange.End.Column
	}

	// Check in range across multiple lines.
	return bpPos.Line > bpRange.Start.Line && bpPos.Line < bpRange.End.Line
}

func rangeToLSPRange(bpRange *source.Range) *lsp.Range {
	if bpRange == nil {
		return nil
	}

	return &lsp.Range{
		Start: lsp.Position{
			Line:      uint32(bpRange.Start.Line - 1),
			Character: uint32(bpRange.Start.Column - 1),
		},
		End: lsp.Position{
			Line:      uint32(bpRange.End.Line - 1),
			Character: uint32(bpRange.End.Column - 1),
		},
	}
}

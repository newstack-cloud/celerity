package languageservices

import (
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
	"go.uber.org/zap"
)

func containsLSPPoint(bpRange *source.Range, lspPos lsp.Position, columnLeeway int) bool {
	bpPos := &source.Position{
		Line:   int(lspPos.Line + 1),
		Column: int(lspPos.Character + 1),
	}

	if bpRange.End == nil {
		return bpPos.Line > bpRange.Start.Line ||
			(bpPos.Line == bpRange.Start.Line && bpPos.Column >= bpRange.Start.Column-columnLeeway)
	}

	// Check in range on single line.
	if bpPos.Line == bpRange.Start.Line && bpPos.Line == bpRange.End.Line {
		return bpPos.Column >= bpRange.Start.Column-columnLeeway &&
			bpPos.Column <= bpRange.End.Column+columnLeeway
	}

	// Check in range on start line.
	if bpPos.Line == bpRange.Start.Line {
		return bpPos.Column >= bpRange.Start.Column-columnLeeway
	}

	// Check in range on end line.
	if bpPos.Line == bpRange.End.Line {
		return bpPos.Column <= bpRange.End.Column+columnLeeway
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

func collectElementsAtPosition(
	tree *schema.TreeNode,
	pos lsp.Position,
	logger *zap.Logger,
	collected *[]*schema.TreeNode,
	columnLeeway int,
) {
	if tree == nil {
		return
	}

	logger.Debug(
		"collectElementsAtPosition: checking element",
		zap.String("path", tree.Path),
		zap.Any("range", tree.Range),
		zap.Any("pos", pos),
	)
	if containsLSPPoint(tree.Range, pos, columnLeeway) {
		logger.Debug(
			"collectElementsAtPosition: found element",
			zap.String("path", tree.Path),
			zap.Any("range", tree.Range),
		)
		logger.Debug("Children length", zap.Int("length", len(tree.Children)))
		*collected = append(*collected, tree)
		i := 0
		for i < len(tree.Children) {
			collectElementsAtPosition(tree.Children[i], pos, logger, collected, columnLeeway)
			i += 1
		}
	}
}

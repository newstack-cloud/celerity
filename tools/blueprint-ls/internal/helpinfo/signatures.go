package helpinfo

import (
	"strings"

	lsp "github.com/newstack-cloud/ls-builder/lsp_3_17"
)

// CustomRenderSignatures renders a list of signatures as a markdown string.
func CustomRenderSignatures(signatures []*lsp.SignatureInformation) string {
	var sb strings.Builder
	for i, sig := range signatures {
		if i > 0 {
			sb.WriteString("\n\n---\n\n")
		}

		sb.WriteString("```")
		sb.WriteString(sig.Label)
		sb.WriteString("```\n\n")
		if docStr, isDocStr := sig.Documentation.(string); isDocStr {
			sb.WriteString(docStr)
		}
		if docMarkup, isDocMarkup := sig.Documentation.(lsp.MarkupContent); isDocMarkup {
			if docMarkup.Kind == lsp.MarkupKindMarkdown {
				sb.WriteString(docMarkup.Value)
			}
		}
	}

	return sb.String()
}

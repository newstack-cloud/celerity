package shared

import (
	"strings"

	stylespkg "github.com/newstack-cloud/deploy-cli-sdk/styles"
)

// KeyHint represents a keyboard shortcut hint for footer navigation.
type KeyHint struct {
	Key  string
	Desc string
}

// RenderFooter renders a row of key hints using the Key and Muted styles.
func RenderFooter(s *stylespkg.Styles, hints []KeyHint) string {
	var sb strings.Builder
	sb.WriteString("  ")
	for i, h := range hints {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(s.Key.Render(h.Key))
		sb.WriteString(s.Muted.Render(" " + h.Desc))
	}
	return sb.String()
}

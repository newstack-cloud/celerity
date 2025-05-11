package styles

import "github.com/charmbracelet/lipgloss"

// CelerityStyles holds the styles to be used across command TUI components.
type CelerityStyles struct {
	Selected   lipgloss.Style
	Selectable lipgloss.Style
}

// NewCelerityStyles creates a new instance of the styles used in the TUI.
func NewCelerityStyles(r *lipgloss.Renderer) *CelerityStyles {
	return &CelerityStyles{
		Selected:   r.NewStyle().Foreground(lipgloss.Color("#5882e2")).Bold(true),
		Selectable: r.NewStyle().Foreground(lipgloss.Color("#2b63e3")),
	}
}

// NewDefaultCelerityStyles creates a new instance of the styles used in the TUI
// with the default renderer.
func NewDefaultCelerityStyles() *CelerityStyles {
	return NewCelerityStyles(lipgloss.DefaultRenderer())
}

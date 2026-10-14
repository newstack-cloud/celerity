package configui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devconfig"
	"github.com/newstack-cloud/celerity/apps/cli/internal/tui/shared"
	styles "github.com/newstack-cloud/deploy-cli-sdk/styles"
)

// EnvSelectModel presents a selectable list of discovered environments.
type EnvSelectModel struct {
	list     list.Model
	styles   *styles.Styles
	selected bool
	choice   devconfig.EnvironmentInfo
}

type envItem struct {
	env devconfig.EnvironmentInfo
}

func (e envItem) FilterValue() string { return e.env.Name }
func (e envItem) Title() string       { return e.env.Name }
func (e envItem) Description() string {
	parts := []string{e.env.DeployTarget}
	if e.env.Region != "" {
		parts = append(parts, e.env.Region)
	}
	return strings.Join(parts, ", ")
}

// NewEnvSelectModel creates a new environment selection model.
func NewEnvSelectModel(
	envs []devconfig.EnvironmentInfo,
	s *styles.Styles,
) EnvSelectModel {
	items := make([]list.Item, len(envs))
	for i, env := range envs {
		items[i] = envItem{env: env}
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 60, 14)
	l.Title = "Select Environment"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(len(items) > 5)
	l.SetShowHelp(false)
	l.Styles.Title = s.Title

	return EnvSelectModel{list: l, styles: s}
}

// Init implements tea.Model.
func (m EnvSelectModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m EnvSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 6)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			// Only while the list is not filtering, where a q is a character
			// the developer is typing into the filter rather than a command.
			if m.list.FilterState() != list.Filtering {
				return m, tea.Quit
			}
		case "enter":
			item, ok := m.list.SelectedItem().(envItem)
			if ok {
				m.selected = true
				m.choice = item.env
				return m, func() tea.Msg {
					return EnvSelectedMsg{Env: item.env}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m EnvSelectModel) View() string {
	if m.selected {
		return ""
	}

	footer := shared.RenderFooter(m.styles, []shared.KeyHint{
		{Key: "↑/↓", Desc: "navigate"},
		{Key: "enter", Desc: "select"},
		{Key: "q", Desc: "quit"},
	})

	return "\n" + m.list.View() + "\n" + footer + "\n"
}

// DispatchHeadlessEnvOutput writes the environment list to a writer.
func DispatchHeadlessEnvOutput(
	w io.Writer,
	envs []devconfig.EnvironmentInfo,
	err error,
) {
	if err != nil {
		fmt.Fprintf(w, "Error discovering environments: %v\n", err)
		return
	}
	if len(envs) == 0 {
		fmt.Fprintln(w, "No environments found.")
		return
	}
	fmt.Fprintln(w, "Available Environments:")
	for _, env := range envs {
		region := env.Region
		if region == "" {
			region = "default"
		}
		fmt.Fprintf(w, "  %s (%s, %s)\n", env.Name, env.DeployTarget, region)
	}
}

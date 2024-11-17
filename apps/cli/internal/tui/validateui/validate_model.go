package validateui

import (
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/deploy-engine/core"
)

var (
	validateCategroyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4f46e5"))
	diagnosticLevelErrorStyle = lipgloss.NewStyle().MarginLeft(2).Foreground(lipgloss.Color("#dc2626"))
	diagnosticLevelWarnStyle  = lipgloss.NewStyle().MarginLeft(2).Foreground(lipgloss.Color("#f97316"))
	diagnosticLevelInfoStyle  = lipgloss.NewStyle().MarginLeft(2).Foreground(lipgloss.Color("#2563eb"))
	diagnosticMessageStyle    = lipgloss.NewStyle().MarginLeft(2)
	locationStyle             = lipgloss.NewStyle().MarginLeft(2).Foreground(lipgloss.Color("#4f46e5"))
)

type ValidateResultMsg *core.ValidateResult

type ValidateErrMsg struct {
	err error
}

type ValidateStreamMsg struct{}

type item struct {
	result     *core.ValidateResult
	filterText string
}

func (i item) FilterValue() string {
	return i.filterText
}

type ValidateModel struct {
	spinner       spinner.Model
	list          list.Model
	engine        core.DeployEngine
	blueprintFile string
	resultStream  chan *core.ValidateResult
	collected     []*core.ValidateResult
	errStream     chan error
	streaming     bool
	err           error
	width         int
	finished      bool
}

func (m ValidateModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m ValidateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case SelectBlueprintMsg:
		m.blueprintFile = msg.blueprintFile
		// SelectBlueprintMsg can be sent multiple times, we need to make sure we aren't collecting
		// duplicate results from the stream by not dispatching commands that will create multiple
		// consumers.
		if !m.streaming {
			cmds = append(cmds, startValidateStreamCmd(m), waitForNextResultCmd(m), checkForErrCmd(m))
		}
		m.streaming = true
	case ValidateResultMsg:
		if msg == nil {
			m.finished = true
			return m, tea.Quit
		}
		m.collected = append(m.collected, msg)
		setListItemsCmd := m.list.SetItems(listItemsFromResults(m.collected))
		cmds = append(cmds, setListItemsCmd, waitForNextResultCmd(m), checkForErrCmd(m))
	case ValidateErrMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
	}

	var spinnerCmd tea.Cmd
	m.spinner, spinnerCmd = m.spinner.Update(msg)
	var listCmd tea.Cmd
	m.list, listCmd = m.list.Update(msg)
	cmds = append(cmds, spinnerCmd, listCmd)

	return m, tea.Batch(cmds...)
}

func (m ValidateModel) View() string {
	log.Printf("ValidateModel: Rendering view m.collected length: %d", len(m.collected))
	if m.err != nil {
		return diagnosticLevelErrorStyle.Render(m.err.Error())
	}

	sb := strings.Builder{}

	for _, result := range m.collected {
		containerStyle := lipgloss.NewStyle().Padding(1, 1).Width(m.width)

		itemSB := strings.Builder{}
		itemSB.WriteString(validateCategroyStyle.Render(string(result.Category)))
		if result.Diagnostic.Level == bpcore.DiagnosticLevelError {
			itemSB.WriteString(
				diagnosticLevelErrorStyle.Render(
					diagnosticLevelName(result.Diagnostic.Level),
				),
			)
		} else if result.Diagnostic.Level == bpcore.DiagnosticLevelWarning {
			itemSB.WriteString(
				diagnosticLevelWarnStyle.Render(
					diagnosticLevelName(result.Diagnostic.Level),
				),
			)
		} else if result.Diagnostic.Level == bpcore.DiagnosticLevelInfo {
			itemSB.WriteString(
				diagnosticLevelInfoStyle.Render(
					diagnosticLevelName(result.Diagnostic.Level),
				),
			)
		}
		itemSB.WriteString(diagnosticMessageStyle.Render(result.Diagnostic.Message))
		if hasPreciseRange(result.Diagnostic.Range) {
			itemSB.WriteString(
				locationStyle.Render(
					fmt.Sprintf("(line %d, column %d)", result.Diagnostic.Range.Start.Line, result.Diagnostic.Range.Start.Column),
				),
			)
		}
		containerRendered := containerStyle.Render(itemSB.String())
		sb.WriteString(containerRendered)
		sb.WriteString("\n")
	}
	if !m.finished {
		sb.WriteString(fmt.Sprintf("\n\n %s Validating project...\n\n", m.spinner.View()))
	}
	return sb.String()
}

func NewValidateModel(engine core.DeployEngine) ValidateModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return ValidateModel{
		spinner:      s,
		engine:       engine,
		list:         list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		resultStream: make(chan *core.ValidateResult),
		errStream:    make(chan error),
	}
}

func diagnosticLevelName(level bpcore.DiagnosticLevel) string {
	switch level {
	case bpcore.DiagnosticLevelError:
		return "error"
	case bpcore.DiagnosticLevelWarning:
		return "warning"
	case bpcore.DiagnosticLevelInfo:
		return "info"
	default:
		return "unknown"
	}
}

func hasPreciseRange(r *bpcore.DiagnosticRange) bool {
	return r != nil && r.Start.Line > 0 && r.Start.Column > 0
}

func listItemsFromResults(results []*core.ValidateResult) []list.Item {
	items := []list.Item{}
	for _, result := range results {
		items = append(items, item{
			result:     result,
			filterText: resultToPlainText(result),
		})
	}
	return items
}

func resultToPlainText(result *core.ValidateResult) string {
	sb := strings.Builder{}
	sb.WriteString(string(result.Category))
	sb.WriteString(" ")
	sb.WriteString(diagnosticLevelName(result.Diagnostic.Level))
	sb.WriteString(" ")
	sb.WriteString(result.Diagnostic.Message)
	if hasPreciseRange(result.Diagnostic.Range) {
		sb.WriteString(fmt.Sprintf(" (line %d, column %d)", result.Diagnostic.Range.Start.Line, result.Diagnostic.Range.Start.Column))
	}
	return sb.String()
}

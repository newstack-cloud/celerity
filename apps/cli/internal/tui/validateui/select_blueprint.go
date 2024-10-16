package validateui

import (
	"errors"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#4f46e5")).Bold(true)
)

type SelectBlueprintMsg struct {
	blueprintFile string
}

type ClearSelectedBlueprintMsg struct{}

type SelectBlueprintModel struct {
	filepicker   filepicker.Model
	selectedFile string
	autoValidate bool
	quitting     bool
	err          error
}

type clearErrorMsg struct{}

func clearErrorAfter(t time.Duration) tea.Cmd {
	return tea.Tick(t, func(_ time.Time) tea.Msg {
		return clearErrorMsg{}
	})
}

func (m SelectBlueprintModel) Init() tea.Cmd {
	fcmd := m.filepicker.Init()
	if m.autoValidate {
		// Dispatch command to select the blueprint file
		// so the validation model can trigger the validation process.
		return tea.Batch(fcmd, selectBlueprintCmd(m.selectedFile))
	}
	return fcmd
}

func (m SelectBlueprintModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		var cmd tea.Cmd
		m.filepicker, cmd = m.filepicker.Update(msg)
		cmds = append(cmds, cmd)
	case clearErrorMsg:
		m.err = nil
	}

	var cmd tea.Cmd
	m.filepicker, cmd = m.filepicker.Update(msg)
	cmds = append(cmds, cmd)

	// Did the user select a file?
	if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
		m.selectedFile = path
		// Dispatch comamand with the path of the selected file.
		cmds = append(cmds, selectBlueprintCmd(path))
	}

	// Did the user select a disabled file?
	// This is only necessary to display an error to the user.
	if didSelect, path := m.filepicker.DidSelectDisabledFile(msg); didSelect {
		// Let's clear the selectedFile and display an error.
		m.err = errors.New(path + " is not valid.")
		m.selectedFile = ""
		return m, tea.Batch(cmd, clearErrorAfter(2*time.Second), clearSelectedBlueprintCmd())
	}

	return m, tea.Batch(cmds...)
}

func (m SelectBlueprintModel) View() string {
	if m.quitting {
		return ""
	}
	var s strings.Builder
	s.WriteString("\n  ")
	if m.err != nil {
		s.WriteString(m.filepicker.Styles.DisabledFile.Render(m.err.Error()))
	} else if m.selectedFile == "" {
		s.WriteString("Pick a blueprint file:")
	} else {
		s.WriteString("Blueprint file: " + selectedStyle.Render(m.selectedFile))
	}
	s.WriteString("\n\n" + m.filepicker.View() + "\n")
	return s.String()
}

func NewSelectBlueprint(blueprintFile string, autoValidate bool) (*SelectBlueprintModel, error) {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".yaml", ".yml", ".json"}
	return &SelectBlueprintModel{
		filepicker:   fp,
		autoValidate: autoValidate,
		selectedFile: blueprintFile,
	}, nil
}

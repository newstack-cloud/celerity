package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct{}

type ValidateMsg struct{}

func (m Model) Init() tea.Cmd {
	return doSomething
}

func doSomething() tea.Msg {
	return "I've done something!"
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ValidateMsg:
		return m, validate()
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlBackslash:
			return m, tea.Quit
		}
	default:
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	return "View\n"
}

func validate() tea.Cmd {
	return func() tea.Msg {
		return "I've validated something!"
	}
}

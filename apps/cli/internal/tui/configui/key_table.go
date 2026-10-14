package configui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/newstack-cloud/celerity/apps/cli/internal/appconfig"
	"github.com/newstack-cloud/celerity/apps/cli/internal/tui/shared"
	styles "github.com/newstack-cloud/deploy-cli-sdk/styles"
)

type keyTableState uint32

const (
	keyTableViewing keyTableState = iota
	keyTableAdding
	keyTableEditing
	keyTableConfirmDelete
)

// KeyTableModel displays and manages config entries with inline editing.
type KeyTableModel struct {
	state       keyTableState
	ctx         context.Context
	entries     []appconfig.ConfigEntry
	store       appconfig.ConfigStore
	storeName   string
	styles      *styles.Styles
	cursor      int
	showSecrets bool

	keyInput   textinput.Model
	valueInput textinput.Model

	width int
}

// Defaults to context.Background() when no context was supplied, which happens
// when the model is constructed directly in tests.
func (m KeyTableModel) reqCtx() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	return context.Background()
}

// NewKeyTableModel creates a new key table model.
func NewKeyTableModel(
	ctx context.Context,
	entries []appconfig.ConfigEntry,
	store appconfig.ConfigStore,
	storeName string,
	s *styles.Styles,
) KeyTableModel {
	ki := textinput.New()
	ki.Placeholder = "key name"
	ki.CharLimit = 128

	vi := textinput.New()
	vi.Placeholder = "value"
	vi.CharLimit = 4096

	return KeyTableModel{
		ctx:        ctx,
		state:      keyTableViewing,
		entries:    entries,
		store:      store,
		storeName:  storeName,
		styles:     s,
		keyInput:   ki,
		valueInput: vi,
	}
}

// Init implements tea.Model.
func (m KeyTableModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m KeyTableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case keyTableViewing:
		return m.updateViewing(msg)
	case keyTableAdding:
		return m.updateInputting(msg, true)
	case keyTableEditing:
		return m.updateInputting(msg, false)
	case keyTableConfirmDelete:
		return m.updateConfirmDelete(msg)
	}
	return m, nil
}

func (m KeyTableModel) updateViewing(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case KeysLoadedMsg:
		m.entries = msg.Entries
		return m, nil
	case tea.KeyMsg:
		return m.handleViewingKey(msg)
	}
	return m, nil
}

func (m KeyTableModel) handleViewingKey(
	msg tea.KeyMsg,
) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}
	case "s":
		m.showSecrets = !m.showSecrets
	case "a":
		m.state = keyTableAdding
		m.keyInput.Reset()
		m.valueInput.Reset()
		m.keyInput.Focus()
		return m, m.keyInput.Cursor.BlinkCmd()
	case "e":
		if len(m.entries) > 0 {
			entry := m.entries[m.cursor]
			m.state = keyTableEditing
			m.keyInput.SetValue(entry.Key)
			m.valueInput.SetValue(entry.Value)
			m.valueInput.Focus()
			return m, m.valueInput.Cursor.BlinkCmd()
		}
	case "d":
		if len(m.entries) > 0 {
			m.state = keyTableConfirmDelete
		}
	case "b":
		return m, func() tea.Msg { return backToStoresMsg{} }
	}
	return m, nil
}

type backToStoresMsg struct{}

func (m KeyTableModel) updateInputting(
	msg tea.Msg,
	isAdd bool,
) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.state = keyTableViewing
			return m, nil
		case "tab":
			if m.keyInput.Focused() {
				m.keyInput.Blur()
				m.valueInput.Focus()
				return m, m.valueInput.Cursor.BlinkCmd()
			}
		case "enter":
			key := m.keyInput.Value()
			value := m.valueInput.Value()
			if key == "" || value == "" {
				return m, nil
			}
			m.state = keyTableViewing
			sensitive := !isPlaintext(key, m.entries)
			return m, SetKeyCmd(m.reqCtx(), m.store, key, value, sensitive)
		}
	case KeySetMsg:
		return m, LoadKeysCmd(m.reqCtx(), m.store)
	}

	var cmd tea.Cmd
	if m.keyInput.Focused() {
		m.keyInput, cmd = m.keyInput.Update(msg)
	} else {
		m.valueInput, cmd = m.valueInput.Update(msg)
	}
	return m, cmd
}

func (m KeyTableModel) updateConfirmDelete(
	msg tea.Msg,
) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			key := m.entries[m.cursor].Key
			m.state = keyTableViewing
			return m, DeleteKeyCmd(m.reqCtx(), m.store, key)
		default:
			m.state = keyTableViewing
		}
	case KeyDeletedMsg:
		return m, LoadKeysCmd(m.reqCtx(), m.store)
	}
	return m, nil
}

// View implements tea.Model.
func (m KeyTableModel) View() string {
	var b strings.Builder

	title := m.styles.Title.Render(fmt.Sprintf("Keys in %s", m.storeName))
	b.WriteString("\n" + title + "\n\n")

	if len(m.entries) == 0 {
		b.WriteString(m.styles.Muted.Render("  No keys found.") + "\n")
	} else {
		for i, e := range m.entries {
			cursor := "  "
			if i == m.cursor {
				cursor = m.styles.Key.Render("> ")
			}
			value := e.Value
			if e.Sensitive && !m.showSecrets {
				value = "********"
			}
			entryType := "String"
			if e.Sensitive {
				entryType = "SecureString"
			}
			b.WriteString(fmt.Sprintf(
				"%s%-20s %-20s %s\n",
				cursor, e.Key, value, m.styles.Muted.Render(entryType),
			))
		}
	}

	b.WriteString("\n")
	b.WriteString(m.renderFooter())
	b.WriteString("\n")

	return b.String()
}

func (m KeyTableModel) renderFooter() string {
	switch m.state {
	case keyTableAdding:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf(
			"  Add key: %s  Value: %s\n",
			m.keyInput.View(), m.valueInput.View(),
		))
		sb.WriteString(shared.RenderFooter(m.styles, []shared.KeyHint{
			{Key: "tab", Desc: "switch field"},
			{Key: "enter", Desc: "save"},
			{Key: "esc", Desc: "cancel"},
		}))
		return sb.String()

	case keyTableEditing:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf(
			"  Edit %s: %s\n",
			m.keyInput.Value(), m.valueInput.View(),
		))
		sb.WriteString(shared.RenderFooter(m.styles, []shared.KeyHint{
			{Key: "enter", Desc: "save"},
			{Key: "esc", Desc: "cancel"},
		}))
		return sb.String()

	case keyTableConfirmDelete:
		return fmt.Sprintf(
			"  Delete %s? %s/%s",
			m.styles.Key.Render(m.entries[m.cursor].Key),
			m.styles.Key.Render("y"),
			m.styles.Muted.Render("N"),
		)

	default:
		secretsHint := shared.KeyHint{Key: "s", Desc: "show secrets"}
		if m.showSecrets {
			secretsHint.Desc = "hide secrets"
		}
		return shared.RenderFooter(m.styles, []shared.KeyHint{
			{Key: "↑/↓", Desc: "navigate"},
			{Key: "a", Desc: "add"},
			{Key: "e", Desc: "edit"},
			{Key: "d", Desc: "delete"},
			secretsHint,
			{Key: "b", Desc: "back"},
			{Key: "q", Desc: "quit"},
		})
	}
}

// DispatchHeadlessKeyOutput writes key entries to a writer.
func DispatchHeadlessKeyOutput(
	w io.Writer,
	storeName string,
	entries []appconfig.ConfigEntry,
	showSecrets bool,
	err error,
) {
	if err != nil {
		fmt.Fprintf(w, "Error loading keys: %v\n", err)
		return
	}
	output := NewConfigOutput(w, false)
	fmt.Fprintf(w, "Keys in %s:\n", storeName)
	output.PrintEntryTable(entries, showSecrets)
}

func isPlaintext(key string, entries []appconfig.ConfigEntry) bool {
	for _, e := range entries {
		if e.Key == key {
			return !e.Sensitive
		}
	}
	return false
}

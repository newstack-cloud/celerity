package configui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/newstack-cloud/celerity/apps/cli/internal/appconfig"
	"github.com/newstack-cloud/celerity/apps/cli/internal/tui/shared"
	styles "github.com/newstack-cloud/deploy-cli-sdk/styles"
)

// StoreSelectModel presents a selectable list of config stores.
type StoreSelectModel struct {
	list     list.Model
	styles   *styles.Styles
	selected bool
	choice   appconfig.StoreInfo
}

type storeItem struct {
	store appconfig.StoreInfo
}

func (s storeItem) FilterValue() string { return s.store.StoreName }
func (s storeItem) Title() string       { return s.store.StoreName }
func (s storeItem) Description() string {
	return fmt.Sprintf("%s (%s)", string(s.store.StoreKind), s.store.ResourceName)
}

// NewStoreSelectModel creates a new store selection model.
func NewStoreSelectModel(
	stores []appconfig.StoreInfo,
	s *styles.Styles,
) StoreSelectModel {
	items := make([]list.Item, len(stores))
	for i, store := range stores {
		items[i] = storeItem{store: store}
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 60, 14)
	l.Title = "Select Store"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(len(items) > 5)
	l.SetShowHelp(false)
	l.Styles.Title = s.Title

	return StoreSelectModel{list: l, styles: s}
}

// Init implements tea.Model.
func (m StoreSelectModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m StoreSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			item, ok := m.list.SelectedItem().(storeItem)
			if ok {
				m.selected = true
				m.choice = item.store
				return m, func() tea.Msg {
					return StoreSelectedMsg{Store: item.store}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m StoreSelectModel) View() string {
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

// DispatchHeadlessStoreOutput writes the store list to a writer.
func DispatchHeadlessStoreOutput(
	w io.Writer,
	stores []appconfig.StoreInfo,
	err error,
) {
	if err != nil {
		fmt.Fprintf(w, "Error loading stores: %v\n", err)
		return
	}
	if len(stores) == 0 {
		fmt.Fprintln(w, "No config stores found in blueprint.")
		return
	}
	output := NewConfigOutput(w, false)
	output.PrintStoreList(stores)
}

package configui

import (
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/newstack-cloud/celerity/apps/cli/internal/appconfig"
	styles "github.com/newstack-cloud/deploy-cli-sdk/styles"
)

type listState uint32

const (
	listStateLoading listState = iota
	listStateBrowsing
	listStateDone
)

// ListModelOptions configures the standalone list model.
type ListModelOptions struct {
	Stores         []appconfig.StoreInfo
	Styles         *styles.Styles
	Headless       bool
	HeadlessWriter io.Writer
}

// ListModel is a standalone model for `config list` (no store argument).
// It shows available stores and allows selection for key browsing.
type ListModel struct {
	state       listState
	stores      []appconfig.StoreInfo
	storeSelect tea.Model
	headless    bool
	writer      io.Writer
	Error       error
}

// NewListModel creates a new list model for browsing stores.
func NewListModel(opts ListModelOptions) ListModel {
	model := ListModel{
		stores:   opts.Stores,
		headless: opts.Headless,
		writer:   opts.HeadlessWriter,
	}

	if opts.Headless {
		model.state = listStateDone
	} else {
		model.state = listStateBrowsing
		model.storeSelect = NewStoreSelectModel(opts.Stores, opts.Styles)
	}

	return model
}

// Init implements tea.Model.
func (m ListModel) Init() tea.Cmd {
	if m.headless {
		return func() tea.Msg {
			DispatchHeadlessStoreOutput(m.writer, m.stores, nil)
			return tea.Quit()
		}
	}
	if m.storeSelect != nil {
		return m.storeSelect.Init()
	}
	return nil
}

// Update implements tea.Model.
func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	case StoreSelectedMsg:
		// The standalone list has nowhere to navigate to, so a selection ends
		// the session.
		if m.headless && m.writer != nil {
			fmt.Fprintf(m.writer, "Selected: %s (%s)\n",
				msg.Store.StoreName, msg.Store.StoreKind)
		}
		return m, tea.Quit
	}

	if m.storeSelect != nil {
		var cmd tea.Cmd
		m.storeSelect, cmd = m.storeSelect.Update(msg)
		return m, cmd
	}
	return m, nil
}

// View implements tea.Model.
func (m ListModel) View() string {
	if m.headless {
		return ""
	}
	if m.storeSelect != nil {
		return m.storeSelect.View()
	}
	return ""
}

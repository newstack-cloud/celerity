package configui

import (
	"context"
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/newstack-cloud/celerity/apps/cli/internal/appconfig"
	"github.com/newstack-cloud/celerity/apps/cli/internal/blueprint"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devconfig"
	styles "github.com/newstack-cloud/deploy-cli-sdk/styles"
)

type sessionState uint32

const (
	stateLoadingEnvs sessionState = iota
	stateSelectEnv
	stateLoadingStores
	stateSelectStore
	stateLoadingKeys
	stateViewKeys
	stateDone
)

// MainModelOptions configures the interactive config TUI session.
type MainModelOptions struct {
	// Context is bound to the store operations the session performs so they are
	// cancelled when the command context is cancelled (e.g. on Ctrl+C).
	Context        context.Context
	AppDir         string
	BlueprintFile  string
	Styles         *styles.Styles
	Headless       bool
	HeadlessWriter io.Writer
	// StoreFactory allows injecting a mock ConfigStore for testing.
	// When nil, the real provider factory is used.
	StoreFactory func(ctx context.Context, target string, info appconfig.StoreInfo, opts appconfig.ProviderOpts) (appconfig.ConfigStore, error)
}

// MainModel is the top-level bubbletea model for the interactive config session.
type MainModel struct {
	state    sessionState
	options  MainModelOptions
	quitting bool

	// Discovered data
	envs   []devconfig.EnvironmentInfo
	stores []appconfig.StoreInfo

	// Sub-models
	envSelect   tea.Model
	storeSelect tea.Model
	keyTable    tea.Model

	// Selected state
	selectedEnv   devconfig.EnvironmentInfo
	selectedStore appconfig.StoreInfo
	configStore   appconfig.ConfigStore

	Error error
}

// NewMainModel creates a new interactive config session model.
func NewMainModel(opts MainModelOptions) MainModel {
	return MainModel{
		state:   stateLoadingEnvs,
		options: opts,
	}
}

// Defaults to context.Background() when no context was supplied, which happens
// when the model is constructed directly in tests.
func (m MainModel) reqCtx() context.Context {
	if m.options.Context != nil {
		return m.options.Context
	}
	return context.Background()
}

// Init implements tea.Model.
func (m MainModel) Init() tea.Cmd {
	if m.options.Headless {
		return nil
	}
	return LoadEnvsCmd(m.options.AppDir)
}

// Update implements tea.Model.
func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			if m.state == stateSelectEnv || m.state == stateSelectStore {
				m.quitting = true
				return m, tea.Quit
			}
		}
	case EnvsLoadedMsg:
		return m.handleEnvsLoaded(msg)
	case EnvsLoadErrorMsg:
		m.Error = msg.Err
		return m, tea.Quit
	case EnvSelectedMsg:
		return m.handleEnvSelected(msg)
	case StoresLoadedMsg:
		return m.handleStoresLoaded(msg)
	case StoresLoadErrorMsg:
		m.Error = msg.Err
		return m, tea.Quit
	case StoreSelectedMsg:
		return m.handleStoreSelected(msg)
	case KeysLoadedMsg:
		return m.handleKeysLoaded(msg)
	case KeysLoadErrorMsg:
		m.Error = msg.Err
		return m, tea.Quit
	case backToStoresMsg:
		return m.handleBackToStores()
	}

	return m.updateCurrentModel(msg)
}

func (m MainModel) handleEnvsLoaded(
	msg EnvsLoadedMsg,
) (tea.Model, tea.Cmd) {
	m.envs = msg.Envs
	if len(m.envs) == 0 {
		m.Error = fmt.Errorf("no deploy config files found in %s", m.options.AppDir)
		return m, tea.Quit
	}
	m.state = stateSelectEnv
	m.envSelect = NewEnvSelectModel(m.envs, m.options.Styles)
	return m, m.envSelect.Init()
}

func (m MainModel) handleEnvSelected(
	msg EnvSelectedMsg,
) (tea.Model, tea.Cmd) {
	m.selectedEnv = msg.Env
	m.state = stateLoadingStores
	return m, m.loadBlueprintAndStoresCmd()
}

func (m MainModel) handleStoresLoaded(
	msg StoresLoadedMsg,
) (tea.Model, tea.Cmd) {
	m.stores = msg.Stores
	if len(m.stores) == 0 {
		m.Error = fmt.Errorf("no celerity/config resources found in blueprint")
		return m, tea.Quit
	}
	m.state = stateSelectStore
	m.storeSelect = NewStoreSelectModel(m.stores, m.options.Styles)
	return m, m.storeSelect.Init()
}

func (m MainModel) handleStoreSelected(
	msg StoreSelectedMsg,
) (tea.Model, tea.Cmd) {
	m.selectedStore = msg.Store
	m.state = stateLoadingKeys

	store, err := m.createConfigStore(msg.Store)
	if err != nil {
		m.Error = err
		return m, tea.Quit
	}
	m.configStore = store
	return m, LoadKeysCmd(m.reqCtx(), store)
}

func (m MainModel) handleKeysLoaded(
	msg KeysLoadedMsg,
) (tea.Model, tea.Cmd) {
	resolvedName := m.resolvedStoreName()
	m.state = stateViewKeys
	m.keyTable = NewKeyTableModel(m.reqCtx(), msg.Entries, m.configStore, resolvedName, m.options.Styles)
	return m, nil
}

func (m MainModel) handleBackToStores() (tea.Model, tea.Cmd) {
	m.state = stateSelectStore
	m.storeSelect = NewStoreSelectModel(m.stores, m.options.Styles)
	return m, m.storeSelect.Init()
}

func (m MainModel) updateCurrentModel(
	msg tea.Msg,
) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateSelectEnv:
		if m.envSelect != nil {
			var cmd tea.Cmd
			m.envSelect, cmd = m.envSelect.Update(msg)
			return m, cmd
		}
	case stateSelectStore:
		if m.storeSelect != nil {
			var cmd tea.Cmd
			m.storeSelect, cmd = m.storeSelect.Update(msg)
			return m, cmd
		}
	case stateViewKeys:
		if m.keyTable != nil {
			var cmd tea.Cmd
			m.keyTable, cmd = m.keyTable.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m MainModel) View() string {
	if m.options.Headless {
		return ""
	}
	if m.quitting {
		return ""
	}
	if m.Error != nil {
		return fmt.Sprintf("\n  Error: %v\n", m.Error)
	}

	switch m.state {
	case stateLoadingEnvs:
		return "\n  Discovering environments...\n"
	case stateSelectEnv:
		if m.envSelect != nil {
			return m.envSelect.View()
		}
	case stateLoadingStores:
		return "\n  Loading stores from blueprint...\n"
	case stateSelectStore:
		if m.storeSelect != nil {
			return m.storeSelect.View()
		}
	case stateLoadingKeys:
		return "\n  Loading keys...\n"
	case stateViewKeys:
		if m.keyTable != nil {
			return m.keyTable.View()
		}
	}
	return ""
}

func (m MainModel) loadBlueprintAndStoresCmd() tea.Cmd {
	return func() tea.Msg {
		bpPath := m.options.BlueprintFile
		if bpPath == "" {
			bpPath = "app.blueprint.yaml"
		}
		bp, _, err := blueprint.LoadForDev(bpPath)
		if err != nil {
			return StoresLoadErrorMsg{Err: err}
		}
		stores := appconfig.CollectStoreInfo(bp)
		return StoresLoadedMsg{Stores: stores}
	}
}

func (m MainModel) createConfigStore(
	info appconfig.StoreInfo,
) (appconfig.ConfigStore, error) {
	resolvedName := m.resolvedStoreName()
	resolved := appconfig.StoreInfo{
		ResourceName:  info.ResourceName,
		StoreName:     resolvedName,
		StoreKind:     info.StoreKind,
		PlaintextKeys: info.PlaintextKeys,
	}

	opts := appconfig.ProviderOpts{
		Region:  m.selectedEnv.Region,
		Profile: m.selectedEnv.Profile,
	}

	if m.options.StoreFactory != nil {
		return m.options.StoreFactory(
			m.reqCtx(), m.selectedEnv.DeployTarget, resolved, opts,
		)
	}
	return appconfig.NewConfigStore(
		m.reqCtx(), m.selectedEnv.DeployTarget, resolved, opts,
	)
}

func (m MainModel) resolvedStoreName() string {
	appEnv := m.selectedEnv.AppEnv
	if appEnv == "" {
		appEnv = m.selectedEnv.Name
	}
	return appEnv + "-" + m.selectedStore.StoreName
}

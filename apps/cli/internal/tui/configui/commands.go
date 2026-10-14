package configui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/celerity/apps/cli/internal/appconfig"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devconfig"
)

// LoadEnvsCmd discovers available environments from deploy config files.
func LoadEnvsCmd(appDir string) tea.Cmd {
	return func() tea.Msg {
		envs, err := devconfig.DiscoverEnvironments(appDir)
		if err != nil {
			return EnvsLoadErrorMsg{Err: err}
		}
		return EnvsLoadedMsg{Envs: envs}
	}
}

// LoadStoresCmd extracts config stores from a loaded blueprint.
func LoadStoresCmd(bp *schema.Blueprint) tea.Cmd {
	return func() tea.Msg {
		stores := appconfig.CollectStoreInfo(bp)
		return StoresLoadedMsg{Stores: stores}
	}
}

// LoadKeysCmd lists all entries from a config store.
func LoadKeysCmd(ctx context.Context, store appconfig.ConfigStore) tea.Cmd {
	return func() tea.Msg {
		entries, err := store.List(ctx)
		if err != nil {
			return KeysLoadErrorMsg{Err: err}
		}
		return KeysLoadedMsg{Entries: entries}
	}
}

// SetKeyCmd sets a key/value in a config store.
func SetKeyCmd(
	ctx context.Context,
	store appconfig.ConfigStore,
	key, value string,
	sensitive bool,
) tea.Cmd {
	return func() tea.Msg {
		err := store.Set(ctx, key, value, sensitive)
		if err != nil {
			return OperationErrorMsg{Err: err}
		}
		return KeySetMsg{Key: key}
	}
}

// DeleteKeyCmd deletes a key from a config store.
func DeleteKeyCmd(ctx context.Context, store appconfig.ConfigStore, key string) tea.Cmd {
	return func() tea.Msg {
		err := store.Delete(ctx, key)
		if err != nil {
			return OperationErrorMsg{Err: err}
		}
		return KeyDeletedMsg{Key: key}
	}
}

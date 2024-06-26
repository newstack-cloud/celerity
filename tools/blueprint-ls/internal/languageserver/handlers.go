package languageserver

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/two-hundred/celerity/tools/blueprint-ls/pkg/mappers"
	common "github.com/two-hundred/ls-builder/common"
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
)

func (a *Application) handleInitialise(ctx *common.LSPContext, params *lsp.InitializeParams) (any, error) {
	a.logger.Debug("Initialising server...")
	clientCapabilities := params.Capabilities
	capabilities := a.handler.CreateServerCapabilities()
	capabilities.CompletionProvider = &lsp.CompletionOptions{}

	hasWorkspaceFolderCapability := clientCapabilities.Workspace != nil && clientCapabilities.Workspace.WorkspaceFolders != nil
	a.state.SetWorkspaceFolderCapability(hasWorkspaceFolderCapability)

	hasConfigurationCapability := clientCapabilities.Workspace != nil && clientCapabilities.Workspace.Configuration != nil
	a.state.SetConfigurationCapability(hasConfigurationCapability)

	result := lsp.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &lsp.InitializeResultServerInfo{
			Name:    Name,
			Version: &Version,
		},
	}

	if hasWorkspaceFolderCapability {
		result.Capabilities.Workspace = &lsp.ServerWorkspaceCapabilities{
			WorkspaceFolders: &lsp.WorkspaceFoldersServerCapabilities{
				Supported: &hasWorkspaceFolderCapability,
			},
		}
	}

	return result, nil
}

func (a *Application) handleInitialised(ctx *common.LSPContext, params *lsp.InitializedParams) error {
	if a.state.HasConfigurationCapability() {
		a.handler.SetWorkspaceDidChangeConfigurationHandler(
			a.handleWorkspaceDidChangeConfiguration,
		)
	}
	return nil
}

func (a *Application) handleWorkspaceDidChangeConfiguration(ctx *common.LSPContext, params *lsp.DidChangeConfigurationParams) error {
	if a.state.HasConfigurationCapability() {
		// Reset all the cached document settings.
		a.state.ClearDocSettings()
	}

	return nil
}

func (a *Application) handleTextDocumentDidChange(ctx *common.LSPContext, params *lsp.DidChangeTextDocumentParams) error {
	ctx.Notify("window/logMessage", &lsp.LogMessageParams{
		Type:    lsp.MessageTypeInfo,
		Message: "Text document changed (server received)",
	})
	ValidateTextDocument(ctx, a.state, params, a.logger)
	return nil
}

func GetDocumentSettings(context *common.LSPContext, state *State, uri string) *DocSettings {
	state.lock.Lock()
	defer state.lock.Unlock()

	if settings, ok := state.documentSettings[uri]; ok {
		return settings
	} else {
		configResponse := []DocSettings{}
		context.Call(lsp.MethodWorkspaceConfiguration, lsp.ConfigurationParams{
			Items: []lsp.ConfigurationItem{
				{
					ScopeURI: &uri,
					Section:  &ConfigSection,
				},
			},
		}, &configResponse)
		context.Notify("window/logMessage", &lsp.LogMessageParams{
			Type:    lsp.MessageTypeInfo,
			Message: "document workspace configuration (server received)",
		})

		if len(configResponse) > 0 {
			state.documentSettings[uri] = &configResponse[0]
			return &configResponse[0]
		}
	}

	return &DocSettings{
		Trace: DocTraceSettings{
			Server: "off",
		},
		MaxNumberOfProblems: 100,
	}
}

func ValidateTextDocument(
	context *common.LSPContext,
	state *State,
	changeParams *lsp.DidChangeTextDocumentParams,
	logger *zap.Logger,
) []lsp.Diagnostic {
	var diagnostics []lsp.Diagnostic
	settings := GetDocumentSettings(context, state, changeParams.TextDocument.URI)
	logger.Debug(fmt.Sprintf("Settings: %v", settings))
	return diagnostics
}

func (a *Application) handleShutdown(ctx *common.LSPContext) error {
	a.logger.Info("Shutting down server...")
	return nil
}

func (a *Application) handleTextDocumentCompletion(context *common.LSPContext, params *lsp.CompletionParams) (interface{}, error) {
	var completionItems []lsp.CompletionItem

	for word, emoji := range mappers.EmojiMapper {
		emojiCopy := emoji // Create a copy of emoji
		completionItems = append(completionItems, lsp.CompletionItem{
			Label:      word,
			Detail:     &emojiCopy,
			InsertText: &emojiCopy,
		})
	}

	return completionItems, nil
}

package languageserver

import (
	"fmt"

	"github.com/tliron/commonlog"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/two-hundred/celerity/tools/blueprint-ls/pkg/mappers"
)

func (a *Application) handleInitialise(ctx *glsp.Context, params *protocol.InitializeParams) (any, error) {
	commonlog.NewInfoMessage(0, "Initialising server...")
	clientCapabilities := params.Capabilities
	capabilities := a.handler.CreateServerCapabilities()
	capabilities.CompletionProvider = &protocol.CompletionOptions{}

	hasWorkspaceFolderCapability := clientCapabilities.Workspace != nil && clientCapabilities.Workspace.WorkspaceFolders != nil
	a.state.SetWorkspaceFolderCapability(hasWorkspaceFolderCapability)

	hasConfigurationCapability := clientCapabilities.Workspace != nil && clientCapabilities.Workspace.Configuration != nil
	a.state.SetConfigurationCapability(hasConfigurationCapability)

	result := protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    Name,
			Version: &Version,
		},
	}

	if hasWorkspaceFolderCapability {
		result.Capabilities.Workspace = &protocol.ServerCapabilitiesWorkspace{
			WorkspaceFolders: &protocol.WorkspaceFoldersServerCapabilities{
				Supported: &hasWorkspaceFolderCapability,
			},
		}
	}

	return result, nil
}

func (a *Application) handleInitialised(ctx *glsp.Context, params *protocol.InitializedParams) error {
	commonlog.NewInfoMessage(0, "Server initialised")

	if a.state.HasConfigurationCapability() {
		a.handler.WorkspaceDidChangeConfiguration = a.handleWorkspaceDidChangeConfiguration
	}
	return nil
}

func (a *Application) handleWorkspaceDidChangeConfiguration(ctx *glsp.Context, params *protocol.DidChangeConfigurationParams) error {
	commonlog.NewInfoMessage(0, "Configuration changed")

	if a.state.HasConfigurationCapability() {
		// Reset all the cached document settings.
		a.state.ClearDocSettings()
	}

	return nil
}

func (a *Application) handleTextDocumentDidChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	ctx.Notify("window/logMessage", &protocol.LogMessageParams{
		Type:    protocol.MessageTypeInfo,
		Message: "Text document changed (server received)",
	})
	ValidateTextDocument(ctx, a.state, params)
	return nil
}

func GetDocumentSettings(context *glsp.Context, state *State, uri string) *DocSettings {
	state.lock.Lock()
	defer state.lock.Unlock()

	if settings, ok := state.documentSettings[uri]; ok {
		return settings
	} else {
		configResponse := []DocSettings{}
		context.Call(protocol.ServerWorkspaceConfiguration, protocol.ConfigurationParams{
			Items: []protocol.ConfigurationItem{
				{
					ScopeURI: &uri,
					Section:  &ConfigSection,
				},
			},
		}, &configResponse)
		context.Notify("window/logMessage", &protocol.LogMessageParams{
			Type:    protocol.MessageTypeInfo,
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

func ValidateTextDocument(context *glsp.Context, state *State, changeParams *protocol.DidChangeTextDocumentParams) []protocol.Diagnostic {
	var diagnostics []protocol.Diagnostic
	settings := GetDocumentSettings(context, state, changeParams.TextDocument.URI)
	commonlog.NewInfoMessage(0, fmt.Sprintf("Settings: %v", settings))
	return diagnostics
}

func (a *Application) handleShutdown(ctx *glsp.Context) error {
	commonlog.NewInfoMessage(0, "Shutting down server...")
	return nil
}

func (a *Application) handleTextDocumentCompletion(context *glsp.Context, params *protocol.CompletionParams) (interface{}, error) {
	var completionItems []protocol.CompletionItem

	for word, emoji := range mappers.EmojiMapper {
		emojiCopy := emoji // Create a copy of emoji
		completionItems = append(completionItems, protocol.CompletionItem{
			Label:      word,
			Detail:     &emojiCopy,
			InsertText: &emojiCopy,
		})
	}

	return completionItems, nil
}

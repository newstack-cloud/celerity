package languageserver

import (
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/container"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/provider"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/schema"
	"github.com/two-hundred/celerity/libs/blueprint/pkg/transform"
	"github.com/two-hundred/celerity/tools/blueprint-ls/internal/blueprint"
	common "github.com/two-hundred/ls-builder/common"
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
)

func (a *Application) handleInitialise(ctx *common.LSPContext, params *lsp.InitializeParams) (any, error) {
	a.logger.Debug("Initialising server...")
	clientCapabilities := params.Capabilities
	capabilities := a.handler.CreateServerCapabilities()
	capabilities.CompletionProvider = &lsp.CompletionOptions{}
	// Take the first position encoding as the one with the highest priority as per the spec.
	// this language server supports all three position encodings. (UTF-16, UTF-8, UTF-32)
	capabilities.PositionEncoding = params.Capabilities.General.PositionEncodings[0]
	a.state.SetPositionEncodingKind(capabilities.PositionEncoding)

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

func (a *Application) handleTextDocumentDidOpen(ctx *common.LSPContext, params *lsp.DidOpenTextDocumentParams) error {
	ctx.Notify("window/logMessage", &lsp.LogMessageParams{
		Type:    lsp.MessageTypeInfo,
		Message: "Text document opened (server received)",
	})
	a.state.SetDocumentContent(params.TextDocument.URI, params.TextDocument.Text)
	return nil
}

func (a *Application) handleTextDocumentDidClose(ctx *common.LSPContext, params *lsp.DidCloseTextDocumentParams) error {
	ctx.Notify("window/logMessage", &lsp.LogMessageParams{
		Type:    lsp.MessageTypeInfo,
		Message: "Text document closed (server received)",
	})
	return nil
}

func (a *Application) handleTextDocumentDidChange(ctx *common.LSPContext, params *lsp.DidChangeTextDocumentParams) error {
	ctx.Notify("window/logMessage", &lsp.LogMessageParams{
		Type:    lsp.MessageTypeInfo,
		Message: "Text document changed (server received)",
	})
	dispatcher := lsp.NewDispatcher(ctx)
	existingContent := a.state.GetDocumentContent(params.TextDocument.URI)
	err := a.saveDocumentContent(params, existingContent)
	if err != nil {
		return err
	}
	diagnostics := ValidateTextDocument(ctx, a.state, params, a.logger)
	// We must push diagnostics even if there are no errors to clear the existing ones
	// in the client.
	err = dispatcher.PublishDiagnostics(lsp.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: diagnostics,
	})
	if err != nil {
		return err
	}
	return nil
}

func (a *Application) saveDocumentContent(params *lsp.DidChangeTextDocumentParams, existingContent *string) error {
	if len(params.ContentChanges) == 0 {
		return nil
	}

	currentContent := ""
	if existingContent != nil {
		currentContent = *existingContent
	}

	for _, change := range params.ContentChanges {
		wholeChange, isWholeChangeEvent := change.(lsp.TextDocumentContentChangeEventWhole)
		if isWholeChangeEvent {
			a.state.SetDocumentContent(params.TextDocument.URI, wholeChange.Text)
			return nil
		}

		change, isChangeEvent := change.(lsp.TextDocumentContentChangeEvent)
		if !isChangeEvent {
			a.logger.Info(fmt.Sprintf("content change event: %+v", change))
			return errors.New(
				"content change event is not of a valid type, expected" +
					" TextDocumentContentChangeEvent or TextDocumentContentChangeEventWhole",
			)
		}

		if change.Range == nil {
			return errors.New("change range is nil")
		}

		startIndex, endIndex := change.Range.IndexesIn(*existingContent, a.state.GetPositionEncodingKind())
		currentContent = currentContent[:startIndex] + change.Text + currentContent[endIndex:]
	}

	a.state.SetDocumentContent(params.TextDocument.URI, currentContent)

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
	diagnostics := []lsp.Diagnostic{}
	settings := GetDocumentSettings(context, state, changeParams.TextDocument.URI)
	logger.Debug(fmt.Sprintf("Settings: %v", settings))
	content := state.GetDocumentContent(changeParams.TextDocument.URI)
	if content == nil {
		return diagnostics
	}

	loader := container.NewDefaultLoader(
		map[string]provider.Provider{},
		map[string]transform.SpecTransformer{},
		nil,
		nil,
		// Disable runtime value validation as it is not needed for diagnostics.
		false,
		// Disable spec transformation as it is not needed for diagnostics.
		false,
	)
	_, err := loader.ValidateString(
		context.Context,
		*content,
		schema.YAMLSpecFormat,
		blueprint.NewParams(
			map[string]map[string]*core.ScalarValue{},
			map[string]*core.ScalarValue{},
			map[string]*core.ScalarValue{},
		),
	)
	if err != nil {
		logger.Error(fmt.Sprintf("Error loading blueprint: %v", err))
		return blueprintErrorToDiagnostics(err, logger)
	}

	return diagnostics
}

func (a *Application) handleShutdown(ctx *common.LSPContext) error {
	a.logger.Info("Shutting down server...")
	return nil
}

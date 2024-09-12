package languageserver

import (
	"errors"
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/davecgh/go-spew/spew"
	"github.com/two-hundred/celerity/libs/blueprint/container"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/tools/blueprint-ls/internal/blueprint"
	common "github.com/two-hundred/ls-builder/common"
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
)

func (a *Application) handleInitialise(ctx *common.LSPContext, params *lsp.InitializeParams) (any, error) {
	a.logger.Debug("Initialising server...")
	clientCapabilities := params.Capabilities
	capabilities := a.handler.CreateServerCapabilities()
	// Take the first position encoding as the one with the highest priority as per the spec.
	// this language server supports all three position encodings. (UTF-16, UTF-8, UTF-32)
	capabilities.PositionEncoding = params.Capabilities.General.PositionEncodings[0]
	a.state.SetPositionEncodingKind(capabilities.PositionEncoding)

	capabilities.SignatureHelpProvider = &lsp.SignatureHelpOptions{
		TriggerCharacters: []string{"(", ","},
	}

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

func (a *Application) handleHover(ctx *common.LSPContext, params *lsp.HoverParams) (*lsp.Hover, error) {
	dispatcher := lsp.NewDispatcher(ctx)

	tree := a.state.GetDocumentTree(params.TextDocument.URI)
	if tree == nil {
		err := a.validateAndPublishDiagnostics(ctx, params.TextDocument.URI, dispatcher)
		if err != nil {
			return nil, err
		}

		tree = a.state.GetDocumentTree(params.TextDocument.URI)
	}

	bpSchema := a.state.GetDocumentSchema(params.TextDocument.URI)
	if bpSchema == nil {
		return nil, errors.New("no schema found for document")
	}

	content, err := GetHoverContent(
		ctx,
		tree,
		bpSchema,
		&params.TextDocumentPositionParams,
		a.functionRegistry,
		a.resourceRegistry,
		a.dataSourceRegistry,
		a.logger,
	)
	if err != nil {
		return nil, err
	}

	if content == nil {
		return nil, nil
	}

	return &lsp.Hover{
		Contents: lsp.MarkupContent{
			Kind:  lsp.MarkupKindMarkdown,
			Value: content.Value,
		},
		Range: content.Range,
	}, nil
}

func (a *Application) handleSignatureHelp(ctx *common.LSPContext, params *lsp.SignatureHelpParams) (*lsp.SignatureHelp, error) {
	dispatcher := lsp.NewDispatcher(ctx)

	tree := a.state.GetDocumentTree(params.TextDocument.URI)
	if tree == nil {
		err := a.validateAndPublishDiagnostics(ctx, params.TextDocument.URI, dispatcher)
		if err != nil {
			return nil, err
		}

		tree = a.state.GetDocumentTree(params.TextDocument.URI)
	}

	signatures, err := GetFunctionSignatures(
		ctx,
		tree,
		&params.TextDocumentPositionParams,
		a.functionRegistry,
		a.logger,
	)
	if err != nil {
		return nil, err
	}

	return &lsp.SignatureHelp{
		Signatures: signatures,
	}, nil
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
	err = a.validateAndPublishDiagnostics(ctx, params.TextDocument.URI, dispatcher)
	return err
}

func (a *Application) validateAndPublishDiagnostics(
	ctx *common.LSPContext,
	uri lsp.URI,
	dispatcher *lsp.Dispatcher,
) error {
	diagnostics, blueprint := ValidateTextDocument(ctx, a.state, uri, a.blueprintLoader, a.logger)
	a.storeDocumentAndDerivedStructures(uri, blueprint)

	// We must push diagnostics even if there are no errors to clear the existing ones
	// in the client.
	err := dispatcher.PublishDiagnostics(lsp.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
	if err != nil {
		return err
	}
	return nil
}

func (a *Application) storeDocumentAndDerivedStructures(
	uri lsp.URI,
	parsed *schema.Blueprint,
) {
	if parsed == nil {
		return
	}
	a.state.SetDocumentSchema(uri, parsed)
	tree := schema.SchemaToTree(parsed)
	a.state.SetDocumentTree(uri, tree)
	a.state.SetDocumentPositionMap(
		uri,
		blueprint.CreatePositionMap(tree),
	)
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
	docURI lsp.URI,
	loader container.Loader,
	logger *zap.Logger,
) ([]lsp.Diagnostic, *schema.Blueprint) {
	diagnostics := []lsp.Diagnostic{}
	settings := GetDocumentSettings(context, state, docURI)
	logger.Debug(fmt.Sprintf("Settings: %v", settings))
	content := state.GetDocumentContent(docURI)
	if content == nil {
		return diagnostics, nil
	}

	validationResult, err := loader.ValidateString(
		context.Context,
		*content,
		schema.YAMLSpecFormat,
		blueprint.NewParams(
			map[string]map[string]*core.ScalarValue{},
			map[string]*core.ScalarValue{},
			map[string]*core.ScalarValue{},
		),
	)
	logger.Info("Blueprint diagnostics: ")
	spew.Fdump(os.Stderr, validationResult.Diagnostics)
	diagnostics = append(
		diagnostics,
		lspDiagnosticsFromBlueprintDiagnostics(
			docURI,
			validationResult.Diagnostics,
			state,
		)...,
	)
	if err != nil {
		logger.Error(fmt.Sprintf("Error loading blueprint: %v", err))
		errDiagnostics := blueprintErrorToDiagnostics(
			err,
			docURI,
			state,
			logger,
		)
		diagnostics = append(diagnostics, errDiagnostics...)
	}

	return diagnostics, validationResult.Schema
}

func (a *Application) handleShutdown(ctx *common.LSPContext) error {
	a.logger.Info("Shutting down server...")
	return nil
}

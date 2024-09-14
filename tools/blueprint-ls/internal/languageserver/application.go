package languageserver

import (
	"github.com/two-hundred/celerity/libs/blueprint/container"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/tools/blueprint-ls/internal/languageservices"
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
	"go.uber.org/zap"
)

type Application struct {
	handler            *lsp.Handler
	state              *languageservices.State
	settingsService    *languageservices.SettingsService
	functionRegistry   provider.FunctionRegistry
	resourceRegistry   resourcehelpers.Registry
	dataSourceRegistry provider.DataSourceRegistry
	blueprintLoader    container.Loader
	completionService  *languageservices.CompletionService
	diagnosticService  *languageservices.DiagnosticsService
	signatureService   *languageservices.SignatureService
	hoverService       *languageservices.HoverService
	symbolService      *languageservices.SymbolService
	logger             *zap.Logger
	traceService       *lsp.TraceService
}

func NewApplication(
	state *languageservices.State,
	settingsService *languageservices.SettingsService,
	traceService *lsp.TraceService,
	functionRegistry provider.FunctionRegistry,
	resourceRegistry resourcehelpers.Registry,
	dataSourceRegistry provider.DataSourceRegistry,
	blueprintLoader container.Loader,
	completionService *languageservices.CompletionService,
	diagnosticService *languageservices.DiagnosticsService,
	signatureService *languageservices.SignatureService,
	hoverService *languageservices.HoverService,
	symbolService *languageservices.SymbolService,
	logger *zap.Logger,
) *Application {
	return &Application{
		state:              state,
		settingsService:    settingsService,
		traceService:       traceService,
		functionRegistry:   functionRegistry,
		resourceRegistry:   resourceRegistry,
		dataSourceRegistry: dataSourceRegistry,
		blueprintLoader:    blueprintLoader,
		completionService:  completionService,
		diagnosticService:  diagnosticService,
		signatureService:   signatureService,
		hoverService:       hoverService,
		symbolService:      symbolService,
		logger:             logger,
	}
}

func (a *Application) Setup() {
	a.handler = lsp.NewHandler(
		lsp.WithInitializeHandler(a.handleInitialise),
		lsp.WithInitializedHandler(a.handleInitialised),
		lsp.WithShutdownHandler(a.handleShutdown),
		lsp.WithTextDocumentDidOpenHandler(a.handleTextDocumentDidOpen),
		lsp.WithTextDocumentDidCloseHandler(a.handleTextDocumentDidClose),
		lsp.WithTextDocumentDidChangeHandler(a.handleTextDocumentDidChange),
		lsp.WithSetTraceHandler(a.traceService.CreateSetTraceHandler()),
		lsp.WithHoverHandler(a.handleHover),
		lsp.WithSignatureHelpHandler(a.handleSignatureHelp),
		lsp.WithCompletionHandler(a.handleCompletion),
		lsp.WithCompletionItemResolveHandler(a.handleCompletionItemResolve),
		lsp.WithDocumentSymbolHandler(a.handleDocumentSymbols),
	)
}

func (a *Application) Handler() *lsp.Handler {
	return a.handler
}

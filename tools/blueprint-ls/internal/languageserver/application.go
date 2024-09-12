package languageserver

import (
	"github.com/two-hundred/celerity/libs/blueprint/container"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
	"go.uber.org/zap"
)

type Application struct {
	handler            *lsp.Handler
	state              *State
	providers          map[string]provider.Provider
	functionRegistry   provider.FunctionRegistry
	resourceRegistry   resourcehelpers.Registry
	dataSourceRegistry provider.DataSourceRegistry
	blueprintLoader    container.Loader
	logger             *zap.Logger
	traceService       *lsp.TraceService
}

func NewApplication(
	state *State,
	traceService *lsp.TraceService,
	providers map[string]provider.Provider,
	transformers map[string]transform.SpecTransformer,
	blueprintLoader container.Loader,
	logger *zap.Logger,
) *Application {
	return &Application{
		state:            state,
		logger:           logger,
		traceService:     traceService,
		providers:        providers,
		blueprintLoader:  blueprintLoader,
		functionRegistry: provider.NewFunctionRegistry(providers),
		resourceRegistry: resourcehelpers.NewRegistry(
			providers,
			transformers,
		),
		dataSourceRegistry: provider.NewDataSourceRegistry(providers),
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
	)
}

func (a *Application) Handler() *lsp.Handler {
	return a.handler
}

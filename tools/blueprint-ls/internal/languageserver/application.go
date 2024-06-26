package languageserver

import (
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
	"go.uber.org/zap"
)

type Application struct {
	handler      *lsp.Handler
	state        *State
	logger       *zap.Logger
	traceService *lsp.TraceService
}

func NewApplication(state *State, traceService *lsp.TraceService, logger *zap.Logger) *Application {
	return &Application{
		state:        state,
		logger:       logger,
		traceService: traceService,
	}
}

func (a *Application) Setup() {
	a.handler = lsp.NewHandler(
		lsp.WithInitializeHandler(a.handleInitialise),
		lsp.WithInitializedHandler(a.handleInitialised),
		lsp.WithShutdownHandler(a.handleShutdown),
		lsp.WithTextDocumentDidChangeHandler(a.handleTextDocumentDidChange),
		lsp.WithCompletionHandler(a.handleTextDocumentCompletion),
		lsp.WithSetTraceHandler(a.traceService.CreateSetTraceHandler()),
	)
}

func (a *Application) Handler() *lsp.Handler {
	return a.handler
}

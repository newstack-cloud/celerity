package languageserver

import (
	"sync"

	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
	"go.uber.org/zap"
)

type Application struct {
	handler    *lsp.Handler
	state      *State
	logger     *zap.Logger
	traceValue lsp.TraceValue
	mu         sync.Mutex
}

func NewApplication(state *State, logger *zap.Logger) *Application {
	return &Application{
		state:  state,
		logger: logger,
	}
}

func (a *Application) Setup() {
	a.handler = lsp.NewHandler(
		lsp.WithInitializeHandler(a.handleInitialise),
		lsp.WithInitializedHandler(a.handleInitialised),
		lsp.WithShutdownHandler(a.handleShutdown),
		lsp.WithTextDocumentDidChangeHandler(a.handleTextDocumentDidChange),
		lsp.WithCompletionHandler(a.handleTextDocumentCompletion),
		lsp.WithSetTraceHandler(a.handleSetTrace),
	)
}

func (a *Application) Handler() *lsp.Handler {
	return a.handler
}

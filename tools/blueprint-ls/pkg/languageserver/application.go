package languageserver

import (
	protocol "github.com/tliron/glsp/protocol_3_16"
)

type Application struct {
	handler *protocol.Handler
	state   *State
}

func NewApplication(state *State) *Application {
	return &Application{
		state:   state,
		handler: &protocol.Handler{},
	}
}

func (a *Application) Setup() {
	a.handler.Initialize = a.handleInitialise
	a.handler.Initialized = a.handleInitialised
	a.handler.Shutdown = a.handleShutdown
	a.handler.TextDocumentDidChange = a.handleTextDocumentDidChange
	a.handler.TextDocumentCompletion = a.handleTextDocumentCompletion
}

func (a *Application) Handler() *protocol.Handler {
	return a.handler
}

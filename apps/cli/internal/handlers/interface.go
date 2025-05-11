package handlers

import "context"

// Handler is an interface that defines a method for handling
// a specific task or operation for a CLI command
// in a non-interactive environment (i.e. no TTY).
type Handler interface {
	Handle(ctx context.Context) error
}

// HandlerFunc is a function type that implements the Handler interface.
type HandlerFunc func(ctx context.Context) error

func (h HandlerFunc) Handle(ctx context.Context) error {
	return h(ctx)
}

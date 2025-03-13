package types

import (
	"io"

	"github.com/gorilla/mux"
)

// SetupFunc is a function that is used to set up
// a specific version of the API.
type SetupFunc func(router *mux.Router) (io.WriteCloser, error)

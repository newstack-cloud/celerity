package types

import (
	"io"

	"github.com/gorilla/mux"
	"github.com/two-hundred/celerity/apps/deploy-engine/core"
)

// SetupFunc is a function that is used to set up
// a specific version of the API.
type SetupFunc func(router *mux.Router, config *core.Config) (io.WriteCloser, error)

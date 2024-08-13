package endpointsv1

import (
	"io"

	"github.com/gorilla/mux"
)

func Setup(router *mux.Router) (io.WriteCloser, error) {
	router.HandleFunc("/health", HealthHandler)
	return nil, nil
}

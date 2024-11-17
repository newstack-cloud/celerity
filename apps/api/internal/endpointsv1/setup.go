package endpointsv1

import (
	"io"

	"github.com/gorilla/mux"
	"github.com/two-hundred/celerity/libs/blueprint/container"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
	"github.com/two-hundred/celerity/libs/deploy-engine/core"
	"go.uber.org/zap"
)

func Setup(router *mux.Router) (io.WriteCloser, error) {
	// todo: switch to production logger.
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}

	blueprintLoader := container.NewDefaultLoader(
		/* providers */ nil,
		/* specTransformers */ nil,
		/* stateContainer */ nil,
		validation.NewRefChainCollector,
		container.WithLoaderTransformSpec(false),
		container.WithLoaderValidateAfterTransform(false),
		container.WithLoaderValidateRuntimeValues(false),
	)
	deployEngine := core.NewDefaultDeployEngine(blueprintLoader, logger)
	router.HandleFunc("/health", HealthHandler).Methods("GET")
	validator := &validateHandler{
		deployEngine,
	}
	router.HandleFunc("/validate/stream", validator.StreamHandler).Methods("POST")
	return nil, nil
}

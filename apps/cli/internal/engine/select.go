package engine

import (
	"github.com/two-hundred/celerity/apps/cli/internal/config"
	"github.com/two-hundred/celerity/libs/blueprint/container"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
	"github.com/two-hundred/celerity/libs/deploy-engine/core"
	"go.uber.org/zap"
)

// Select returns the appropriate deploy engine based on the configuration
// provided.
func Select(confProvider *config.Provider, logger *zap.Logger) core.DeployEngine {
	if embedded, _ := confProvider.GetBool("embeddedEngine"); embedded {
		loader := container.NewDefaultLoader(
			map[string]provider.Provider{},
			map[string]transform.SpecTransformer{},
			/* stateContainer */ nil,
			/* resourceChangeStager */ nil,
			/* childResolver */ nil,
			container.WithLoaderTransformSpec(false),
			container.WithLoaderValidateAfterTransform(false),
			container.WithLoaderValidateRuntimeValues(false),
		)
		return core.NewDefaultDeployEngine(loader, logger)
	}
	connectProtocol, _ := confProvider.GetString("connectProtocol")
	return NewEngineAPI(connectProtocol)
}

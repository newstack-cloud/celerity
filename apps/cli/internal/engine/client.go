package engine

import (
	"github.com/two-hundred/celerity/apps/cli/internal/config"
	deployengine "github.com/two-hundred/celerity/libs/deploy-engine-client"
	"go.uber.org/zap"
)

// Create a new deploy engine client based on how the CLI is configured.
func Create(confProvider *config.Provider, logger *zap.Logger) (DeployEngine, error) {
	return deployengine.NewClient()
}

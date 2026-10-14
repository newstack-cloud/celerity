// Command maintenance-runner is the in-network half of Celerity's database
// maintenance path.
//
// It is deployed alongside a celerity/sqlDatabase, on the same private network,
// and performs operations the CLI cannot perform from outside it.
//
// The only platform-specific thing here is which host packages are linked in.
// Each registers itself, the runner serves on whichever one it detects at
// startup, and adding a platform means adding an import.
package main

import (
	"fmt"
	"os"

	"github.com/newstack-cloud/celerity/apps/cli/internal/maintenancerunner"
	"go.uber.org/zap"

	// Host platforms. AWS runs the runner on Lambda, so its binary must be named
	// bootstrap in the deployed archive.
	_ "github.com/newstack-cloud/celerity/apps/cli/internal/maintenancerunner/awsrunner"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	if err := maintenancerunner.Run(logger); err != nil {
		logger.Error("the maintenance runner could not start", zap.Error(err))
		os.Exit(1)
	}
}

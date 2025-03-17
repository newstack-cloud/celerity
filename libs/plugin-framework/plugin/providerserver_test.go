// Integrated test suite for v1 provider plugins.
// This suite is designed to test the full lifecycle of a v1 provider plugin
// including registration and interaction with the host service,
// this is an integrated test that comes close to an end-to-end test,
// the only difference is that the network listener is in-process
// meaning that the host service and provider plugin are running in the same process
// for the automated tests.
package plugin

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ProviderPluginV1Suite struct {
	closePluginService func()
	suite.Suite
}

func (s *ProviderPluginV1Suite) SetupSuite() {
	// pluginService, closePluginService := testutils.StartPluginServiceServer(
	// 	"test-host-id",
	// 	pluginManager,
	// 	functionRegistry,
	// 	resourceDeployService,
	// )
	// s.closePluginService = closePluginService
}

func (s *ProviderPluginV1Suite) TearDownSuite() {
	s.closePluginService()
}

func TestProviderPluginV1Suite(t *testing.T) {
	suite.Run(t, new(ProviderPluginV1Suite))
}

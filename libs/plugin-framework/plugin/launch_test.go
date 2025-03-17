package plugin

import (
	context "context"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/plugin-framework/pluginservicev1"
)

const (
	testTransformName = "celerity-transform-2025"
)

type LaunchSuite struct {
	fs       afero.Fs
	expected []*PluginPathInfo
	launcher *PluginLauncher
	suite.Suite
}

func (s *LaunchSuite) SetupTest() {
	s.expected = loadExpectedPluginPaths()
	s.fs = afero.NewMemMapFs()
	err := loadPluginsIntoFS(s.expected, s.fs)
	s.Require().NoError(err)

	pluginPath := strings.Join(testPluginRootPaths, ":")
	manager := &mockPluginManager{
		pluginMap: map[pluginservicev1.PluginType]map[string]*pluginservicev1.PluginInstance{
			pluginservicev1.PluginType_PLUGIN_TYPE_PROVIDER:    {},
			pluginservicev1.PluginType_PLUGIN_TYPE_TRANSFORMER: {},
		},
		testTransformName: testTransformName,
	}
	executor := &mockPluginExecutor{
		pluginManager: manager,
		registerOnAttempt: map[string]int{
			s.expected[0].AbsolutePath: 1,
			s.expected[1].AbsolutePath: 4,
			s.expected[2].AbsolutePath: 2,
		},
		registerAttempts: map[string]int{},
		pluginInstances:  s.instancesFromPluginPaths(),
	}
	s.launcher = NewPluginLauncher(
		pluginPath,
		manager,
		executor,
		WithPluginLauncherFS(s.fs),
		WithPluginLauncherAttemptLimit(5),
		WithPluginLauncherWaitTimeout(5*time.Millisecond),
		WithPluginLauncherCheckRegisteredInterval(1*time.Millisecond),
	)
}

func (s *LaunchSuite) instancesFromPluginPaths() map[string]*pluginservicev1.PluginInstanceInfo {
	instances := map[string]*pluginservicev1.PluginInstanceInfo{}
	for _, pluginPath := range s.expected {
		instances[pluginPath.AbsolutePath] = &pluginservicev1.PluginInstanceInfo{
			PluginType: pluginservicev1.PluginTypeFromString(
				pluginPath.PluginType,
			),
			ProtocolVersion: 1,
			ID:              pluginPath.ID,
		}
	}
	return instances
}

func (s *LaunchSuite) Test_launches_plugins() {
	pluginMaps, err := s.launcher.Launch(context.Background())
	s.Require().NoError(err)

	s.Assert().Len(pluginMaps.Providers, 2)

	s.assertHasProvider(pluginMaps, "aws")
	s.assertHasProvider(pluginMaps, "azure")

	s.Assert().Len(pluginMaps.Transformers, 1)

	s.assertHasTransformer(pluginMaps, testTransformName)
}

func (s *LaunchSuite) assertHasProvider(
	pluginMaps *PluginMaps,
	namespace string,
) {
	provider, hasProvider := pluginMaps.Providers[namespace]
	s.Assert().True(hasProvider)
	result, err := provider.Namespace(context.Background())
	s.Require().NoError(err)
	s.Assert().Equal(namespace, result)
}

func (s *LaunchSuite) assertHasTransformer(
	pluginMaps *PluginMaps,
	transformName string,
) {
	transformer, hasTransformer := pluginMaps.Transformers[transformName]
	s.Assert().True(hasTransformer)
	result, err := transformer.GetTransformName(context.Background())
	s.Require().NoError(err)
	s.Assert().Equal(transformName, result)
}

func TestLaunchSuite(t *testing.T) {
	suite.Run(t, new(LaunchSuite))
}

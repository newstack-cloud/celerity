package plugin

import (
	context "context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/afero"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
	"github.com/two-hundred/celerity/libs/plugin-framework/plugin/pluginservicev1"
	"github.com/two-hundred/celerity/libs/plugin-framework/utils"
)

const (
	// DefaultPluginLaunchAttemptLimit is the default number of times to attempt
	// launching a plugin before giving up.
	DefaultPluginLaunchAttemptLimit = 5
	// DefaultLaunchWaitTimeout is the default timeout to wait for a plugin to register
	// with the host service.
	DefaultLaunchWaitTimeout = 20 * time.Millisecond
	// DefaultCheckRegisteredInterval is the default interval to check if a plugin has
	// registered with the host service.
	DefaultCheckRegisteredInterval = 5 * time.Millisecond
)

var (
	// ErrPluginRegistrationTimeout is returned when a plugin does not register
	// with the host service within the wait timeout.
	ErrPluginRegistrationTimeout = errors.New("plugin registration timeout")
)

// PluginMaps is a set of adaptors that can be used as maps of providers
// and transformers to be used to create a blueprint loader.
type PluginMaps struct {
	Providers    map[string]provider.Provider
	Transformers map[string]transform.SpecTransformer
}

// PluginLauncher is a service that launches plugins and waits for them to register
// with the host service.
type PluginLauncher struct {
	pluginPath              string
	manager                 pluginservicev1.Manager
	executor                PluginExecutor
	fs                      afero.Fs
	launchAttemptLimit      int
	launchWaitTimeout       time.Duration
	checkRegisteredInterval time.Duration
}

// PluginLauncherOption is a function that configures a PluginLauncher.
type PluginLauncherOption func(*PluginLauncher)

// WithPluginLauncherFS is a PluginLauncher option that sets the file system to use
// when discovering plugins.
func WithPluginLauncherFS(fs afero.Fs) PluginLauncherOption {
	return func(l *PluginLauncher) {
		l.fs = fs
	}
}

// WithPluginLauncherAttemptLimit is a PluginLauncher option that sets the number of times
// to attempt launching a plugin before giving up.
func WithPluginLauncherAttemptLimit(attemptLimit int) PluginLauncherOption {
	return func(l *PluginLauncher) {
		l.launchAttemptLimit = attemptLimit
	}
}

// WithPluginLauncherWaitTimeout is a PluginLauncher option that sets the timeout to wait
// for a plugin to register with the host service.
func WithPluginLauncherWaitTimeout(timeout time.Duration) PluginLauncherOption {
	return func(l *PluginLauncher) {
		l.launchWaitTimeout = timeout
	}
}

// WithPluginLauncherCheckRegisteredInterval is a PluginLauncher option that sets the interval
// to check if a plugin has registered with the host service.
func WithPluginLauncherCheckRegisteredInterval(interval time.Duration) PluginLauncherOption {
	return func(l *PluginLauncher) {
		l.checkRegisteredInterval = interval
	}
}

// NewPluginLauncher creates a new PluginLauncher.
func NewPluginLauncher(
	pluginPath string,
	manager pluginservicev1.Manager,
	executor PluginExecutor,
	opts ...PluginLauncherOption,
) *PluginLauncher {
	launcher := &PluginLauncher{
		pluginPath:              pluginPath,
		manager:                 manager,
		executor:                executor,
		fs:                      afero.NewOsFs(),
		launchAttemptLimit:      DefaultPluginLaunchAttemptLimit,
		launchWaitTimeout:       DefaultLaunchWaitTimeout,
		checkRegisteredInterval: DefaultCheckRegisteredInterval,
	}

	for _, opt := range opts {
		opt(launcher)
	}

	return launcher
}

// Launch discovers, executes plugin binaries
// and waits for the plugins to have registered with the host service.
// This returns a set of adaptors that can be used as maps of providers
// and transformers to be used to create a blueprint loader.
//
// The provided plugin path is expected to be a colon-separated
// list of root directories to search for plugins in.
//
// The provided context should set a deadline to avoid waiting
// indefinitely for plugins to register with the host service.
func (l *PluginLauncher) Launch(ctx context.Context) (*PluginMaps, error) {
	plugins, err := DiscoverPlugins(l.pluginPath, l.fs)
	if err != nil {
		return nil, err
	}

	for _, plugin := range plugins {
		err := l.launchPlugin(ctx, plugin, 1 /* attemptNumber */)
		if err != nil {
			return nil, err
		}
	}

	providerPlugins := l.manager.GetPlugins(pluginservicev1.PluginType_PLUGIN_TYPE_PROVIDER)
	providerPluginMap, err := createProviderPluginAdaptors(providerPlugins)
	if err != nil {
		return nil, err
	}

	transformerPlugins := l.manager.GetPlugins(pluginservicev1.PluginType_PLUGIN_TYPE_TRANSFORMER)
	transformerPluginMap, err := createTransformerPluginAdaptors(ctx, transformerPlugins)
	if err != nil {
		return nil, err
	}

	return &PluginMaps{
		Providers:    providerPluginMap,
		Transformers: transformerPluginMap,
	}, nil
}

func (l *PluginLauncher) launchPlugin(
	ctx context.Context,
	plugin *PluginPathInfo,
	attemptNumber int,
) error {
	pluginProcess, err := l.executor.Execute(plugin.AbsolutePath)
	if err != nil {
		return err
	}

	err = l.waitForPluginRegistration(
		ctx,
		plugin,
		pluginProcess.Kill,
	)
	if err != nil {
		if errors.Is(err, ErrPluginRegistrationTimeout) {
			if attemptNumber <= l.launchAttemptLimit {
				return l.launchPlugin(ctx, plugin, attemptNumber+1)
			}
		}
		return err
	}

	return nil
}

func (l *PluginLauncher) waitForPluginRegistration(
	ctx context.Context,
	plugin *PluginPathInfo,
	stop func() error,
) error {
	startTime := time.Now()
	for time.Since(startTime) < l.launchWaitTimeout {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			pluginInstance := l.manager.GetPlugin(
				pluginservicev1.PluginTypeFromString(plugin.PluginType),
				plugin.ID,
			)
			if pluginInstance != nil {
				return nil
			}
			time.Sleep(l.checkRegisteredInterval)
		}
	}

	err := stop()
	if err != nil {
		return err
	}

	return ErrPluginRegistrationTimeout
}

func createProviderPluginAdaptors(
	providerPlugins []*pluginservicev1.PluginInstance,
) (map[string]provider.Provider, error) {
	providerPluginMap := make(map[string]provider.Provider)
	for _, providerPluginInstance := range providerPlugins {
		providerPlugin, ok := providerPluginInstance.Client.(provider.Provider)
		if !ok {
			return nil, fmt.Errorf(
				"plugin %s is not an instance of provider.Provider",
				providerPluginInstance.Info.ID,
			)
		}
		providerNamespace := utils.ExtractProviderNamespace(providerPluginInstance.Info.ID)
		providerPluginMap[providerNamespace] = providerPlugin
	}
	return providerPluginMap, nil
}

func createTransformerPluginAdaptors(
	ctx context.Context,
	transformerPlugins []*pluginservicev1.PluginInstance,
) (map[string]transform.SpecTransformer, error) {
	transformerPluginMap := make(map[string]transform.SpecTransformer)
	for _, transformerPluginInstance := range transformerPlugins {
		transformerPlugin, ok := transformerPluginInstance.Client.(transform.SpecTransformer)
		if !ok {
			return nil, fmt.Errorf(
				"plugin %s is not an instance of transform.SpecTransformer",
				transformerPluginInstance.Info.ID,
			)
		}

		// For transformers, the string used in a blueprint in the `transform` section
		// is used to resolve the correct transformer plugin to use when loading a blueprint.
		transformName, err := transformerPlugin.GetTransformName(ctx)
		if err != nil {
			return nil, err
		}
		transformerPluginMap[transformName] = transformerPlugin
	}
	return transformerPluginMap, nil
}

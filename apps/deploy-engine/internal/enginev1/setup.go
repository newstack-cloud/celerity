package enginev1

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/afero"
	"github.com/two-hundred/celerity/apps/deploy-engine/core"
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/pluginhostv1"
	"github.com/two-hundred/celerity/libs/blueprint-resolvers/azure"
	resolverfs "github.com/two-hundred/celerity/libs/blueprint-resolvers/fs"
	"github.com/two-hundred/celerity/libs/blueprint-resolvers/gcs"
	resolverrouter "github.com/two-hundred/celerity/libs/blueprint-resolvers/router"
	"github.com/two-hundred/celerity/libs/blueprint-resolvers/s3"
	"github.com/two-hundred/celerity/libs/blueprint/container"
	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/providerhelpers"
	"github.com/two-hundred/celerity/libs/plugin-framework/plugin"
)

func Setup(router *mux.Router, config *core.Config) (io.WriteCloser, error) {
	logger, err := core.CreateLogger(config)
	if err != nil {
		return nil, err
	}

	fileSystem := afero.NewOsFs()
	idGenerator := bpcore.NewUUIDGenerator()

	stateContainer, err := loadStateContainer(
		context.Background(),
		fileSystem,
		logger,
		config.State,
	)
	if err != nil {
		return nil, err
	}

	clock := &bpcore.SystemClock{}
	initialProviders := map[string]provider.Provider{
		"core": providerhelpers.NewCoreProvider(
			stateContainer.Links(),
			bpcore.BlueprintInstanceIDFromContext,
			os.Getwd,
			clock,
		),
	}
	pluginExecutor := plugin.NewOSCmdExecutor(
		config.PluginsV1.LogFileRootDir,
	)
	pluginHostService, err := pluginhostv1.LoadDefaultService(
		&pluginhostv1.LoadDependencies{
			Executor:         pluginExecutor,
			InstanceFactory:  plugin.CreatePluginInstance,
			PluginHostConfig: config,
		},
		pluginhostv1.WithServiceLogger(logger),
		pluginhostv1.WithServiceFS(fileSystem),
		pluginhostv1.WithIDGenerator(idGenerator),
		pluginhostv1.WithInitialProviders(initialProviders),
	)
	if err != nil {
		return nil, err
	}

	pluginMaps, err := pluginHostService.LoadPlugins(
		context.Background(),
	)
	if err != nil {
		return nil, err
	}

	fsResolver := resolverfs.NewResolver(fileSystem)
	s3Resolver := s3.NewResolver(config.Resolvers.S3Endpoint, false)
	gcsResolver := gcs.NewResolver(config.Resolvers.GCSEndpoint)
	// Azure blob storage clients will be created on the fly
	// using default credentials and sourcing the storage account
	// name from include metadata, blueprint params or
	// environment variables.
	azureObjectResolver := azure.NewResolver( /* clientFactory */ nil)
	childResolver := resolverrouter.NewResolver(
		fsResolver,
		resolverrouter.WithRoute("aws/s3", s3Resolver),
		resolverrouter.WithRoute("azure/blob", azureObjectResolver),
		resolverrouter.WithRoute("googlecloud/storage", gcsResolver),
	)

	validateLoader := container.NewDefaultLoader(
		pluginMaps.Providers,
		pluginMaps.Transformers,
		/* stateContainer */ nil,
		/* childResolver */ nil,
		container.WithLoaderTransformSpec(false),
		container.WithLoaderValidateAfterTransform(false),
		container.WithLoaderValidateRuntimeValues(false),
		container.WithLoaderLogger(logger),
	)

	defaultRetryPolicy := parseDefaultRetryPolicy(
		config.Blueprints.DefaultRetryPolicy,
		logger.Named("init"),
	)

	deployLoader := container.NewDefaultLoader(
		pluginMaps.Providers,
		pluginMaps.Transformers,
		stateContainer,
		childResolver,
		container.WithLoaderTransformSpec(true),
		// Sometimes users of the deploy engine will want to debug issues
		// caused by a transformer plugin that may be producing invalid output.
		container.WithLoaderValidateAfterTransform(config.Blueprints.ValidateAfterTransform),
		container.WithLoaderValidateRuntimeValues(true),
		container.WithLoaderDriftCheckEnabled(config.Blueprints.EnableDriftCheck),
		container.WithLoaderIDGenerator(idGenerator),
		container.WithLoaderDefaultRetryPolicy(defaultRetryPolicy),
		container.WithLoaderResourceStabilityPollingConfig(
			createResourceStabilityPollingConfig(config),
		),
		container.WithLoaderLogger(logger),
	)

	deployEngine := core.NewDefaultDeployEngine(validateLoader, deployLoader, logger)

	healthHandler := router.HandleFunc("/health", HealthHandler).Methods("GET")
	validator := &validateHandler{
		deployEngine,
	}
	// TODO: make validate stream a GET request (for SSE)
	router.HandleFunc(
		"/validate/stream",
		validator.StreamHandler,
	).Methods("POST")

	authMiddleware, err := setupAuth(
		config.Auth,
		clock,
		/* excludedRoutes */ []*mux.Route{healthHandler},
	)
	if err != nil {
		return nil, err
	}

	router.Use(authMiddleware.Middleware)

	return nil, nil
}

func createResourceStabilityPollingConfig(
	config *core.Config,
) *container.ResourceStabilityPollingConfig {
	return &container.ResourceStabilityPollingConfig{
		PollingInterval: time.Duration(
			config.Blueprints.ResourceStabilisationPollingIntervalMS,
		) * time.Millisecond,
		PollingTimeout: time.Duration(
			config.PluginsV1.ResourceStabilisationPollingTimeoutMS,
		) * time.Millisecond,
	}
}

func parseDefaultRetryPolicy(
	serialised string,
	logger bpcore.Logger,
) *provider.RetryPolicy {
	if strings.TrimSpace(serialised) == "" {
		return provider.DefaultRetryPolicy
	}

	retryPolicy := &provider.RetryPolicy{}
	err := json.Unmarshal([]byte(serialised), retryPolicy)
	if err != nil {
		logger.Warn(
			"failed to parse default retry policy from config, "+
				"using default policy from the provider package",
			bpcore.ErrorLogField("error", err),
		)
		return provider.DefaultRetryPolicy
	}

	return retryPolicy
}

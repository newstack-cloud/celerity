package devrun

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/celerity/apps/cli/internal/blueprint"
	"github.com/newstack-cloud/celerity/apps/cli/internal/compose"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devlogs"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devstate"
	"github.com/newstack-cloud/celerity/apps/cli/internal/docker"
	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
	"github.com/newstack-cloud/celerity/apps/cli/internal/seed"
	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
	"go.uber.org/zap"
)

const shutdownTimeout = 30 * time.Second

// OrchestratorConfig holds all resolved configuration for a dev run.
type OrchestratorConfig struct {
	AppDir              string
	Port                string
	ServiceName         string
	Image               string
	DeployTarget        string
	Runtime             string
	MergedBlueprintPath string
	ModulePath          string
	SeedDir             string
	ConfigDir           string
	SecretsDir          string

	Blueprint    *schema.Blueprint
	SpecFormat   schema.SpecFormat
	HandlerInfos []blueprint.HandlerInfo
	Manifest     *preprocess.HandlerManifest
	ContainerCfg *docker.ContainerConfig
	ComposeCfg   *compose.ComposeConfig
}

// Orchestrator manages the full dev run lifecycle.
type Orchestrator struct {
	config    OrchestratorConfig
	docker    docker.RuntimeContainerManager
	compose   *compose.ComposeManager
	extractor *preprocess.Extractor
	output    *Output
	logger    *zap.Logger

	containerID string
	manifest    *preprocess.HandlerManifest
}

// NewOrchestrator creates a new dev run orchestrator.
func NewOrchestrator(
	config OrchestratorConfig,
	dockerMgr docker.RuntimeContainerManager,
	composeMgr *compose.ComposeManager,
	extractor *preprocess.Extractor,
	output *Output,
	logger *zap.Logger,
) *Orchestrator {
	return &Orchestrator{
		config:    config,
		docker:    dockerMgr,
		compose:   composeMgr,
		extractor: extractor,
		output:    output,
		logger:    logger,
		manifest:  config.Manifest,
	}
}

// RunForeground starts the dev environment and streams logs until interrupted.
func (o *Orchestrator) RunForeground(ctx context.Context) error {
	if err := o.startup(ctx); err != nil {
		return err
	}

	state := o.buildState(false)
	if err := devstate.Write(o.config.AppDir, state); err != nil {
		o.output.PrintWarning("Failed to write state file", err)
	}

	o.output.PrintStartupSummary(o.config.Port, o.config.HandlerInfos)
	o.output.PrintStreamingNotice()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start file watcher in background.
	go o.startWatcher(ctx)

	// Stream logs in background.
	logDone := make(chan error, 1)
	go func() {
		logDone <- o.streamLogs(ctx)
	}()

	// Wait for shutdown signal or log stream end.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		o.output.PrintShutdownStarting()
	case err := <-logDone:
		if err != nil {
			o.output.PrintWarning("Log stream ended", err)
		}
	}

	cancel()
	return o.Shutdown(context.Background())
}

// RunDetached starts the dev environment and exits, leaving containers running.
func (o *Orchestrator) RunDetached(ctx context.Context) error {
	if err := o.startup(ctx); err != nil {
		return err
	}

	state := o.buildState(true)
	if err := devstate.Write(o.config.AppDir, state); err != nil {
		o.output.PrintWarning("Failed to write state file", err)
	}

	o.output.PrintStartupSummary(o.config.Port, o.config.HandlerInfos)
	o.output.PrintDetachedNotice()

	return nil
}

// Shutdown stops the container, compose stack, and removes the state file.
// Shared between foreground Ctrl+C and the `dev stop` command.
func (o *Orchestrator) Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	if o.containerID != "" {
		if err := o.docker.Stop(ctx, o.containerID); err != nil {
			o.output.PrintWarning("Failed to stop container", err)
		}
	}

	if o.compose != nil && o.compose.HasServices() {
		if err := o.compose.Down(ctx); err != nil {
			o.output.PrintWarning("Failed to stop dependencies", err)
		}
	}

	if err := devstate.Remove(o.config.AppDir); err != nil {
		o.output.PrintWarning("Failed to remove state file", err)
	}

	o.output.PrintShutdownComplete()
	return nil
}

// DumpLogs syncs all container logs to files in .celerity/logs/ before teardown.
// This is called by `dev test` so logs are preserved after the environment is torn down.
// Returns the log directory path, or empty string if no logs were written.
func (o *Orchestrator) DumpLogs(ctx context.Context) string {
	logDir := filepath.Join(celerityDir(o.config.AppDir), "logs")

	// Dump compose service logs (datastore, stubs, valkey, etc.)
	o.dumpComposeLogs(ctx, logDir)

	if o.containerID == "" {
		return logDir
	}

	fileWriter, err := devlogs.NewLogFileWriter(celerityDir(o.config.AppDir))
	if err != nil {
		o.logger.Warn("log file setup failed", zap.Error(err))
		return logDir
	}
	defer fileWriter.Close()

	streamer := devlogs.NewStreamer(o.docker, o.config.Runtime, false)
	streamer.FileWriter = fileWriter

	result, err := streamer.SyncToFiles(ctx, o.containerID, devlogs.StreamOptions{
		Tail: "all",
	})
	if err != nil {
		o.logger.Warn("log dump failed", zap.Error(err))
		return logDir
	}

	o.logger.Debug("dumped container logs",
		zap.String("logDir", result.LogDir),
		zap.Int("lines", result.TotalLines),
		zap.Strings("handlerFiles", result.HandlerFiles),
	)

	return result.LogDir
}

// dumpComposeLogs writes each compose service's logs to a separate file
// in the log directory (e.g. stubs.log, datastore.log, valkey.log).
func (o *Orchestrator) dumpComposeLogs(ctx context.Context, logDir string) {
	if o.compose == nil {
		return
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		o.logger.Warn("failed to create log directory for compose logs", zap.Error(err))
		return
	}

	for _, svc := range o.dependencyServiceNames() {
		logs, err := o.compose.Logs(ctx, svc, 0)
		if err != nil {
			o.logger.Debug("failed to fetch logs for service",
				zap.String("service", svc), zap.Error(err))
			continue
		}
		if logs == "" {
			continue
		}

		path := filepath.Join(logDir, svc+".log")
		if err := os.WriteFile(path, []byte(logs), 0644); err != nil {
			o.logger.Warn("failed to write compose service log",
				zap.String("service", svc), zap.Error(err))
		}
	}
}

// ShutdownFromState cleans up using a loaded state (for `dev stop`).
func (o *Orchestrator) ShutdownFromState(ctx context.Context, state *devstate.DevState) error {
	o.containerID = state.ContainerID
	return o.Shutdown(ctx)
}

// StartFull runs the full startup sequence (dependencies, seeding, container)
// and writes state. Used by dev test when API tests are included.
func (o *Orchestrator) StartFull(ctx context.Context) error {
	if err := o.startup(ctx); err != nil {
		return err
	}
	state := o.buildState(false)
	if err := devstate.Write(o.config.AppDir, state); err != nil {
		o.output.PrintWarning("Failed to write state file", err)
	}
	o.output.PrintStartupSummary(o.config.Port, o.config.HandlerInfos)
	return nil
}

// StartInfraOnly runs compose dependencies, seeds config/data, and applies
// SQL schemas, but does NOT start the app container.
// Used by dev test when only integration tests are requested.
func (o *Orchestrator) StartInfraOnly(ctx context.Context) error {
	if err := o.checkDocker(ctx); err != nil {
		return err
	}

	devlogs.CleanLogDir(celerityDir(o.config.AppDir))

	if err := o.startDependencies(ctx); err != nil {
		return err
	}

	if err := o.seedConfig(ctx); err != nil {
		return err
	}

	// hostMode=true: stub URLs use localhost since tests run on the host.
	if err := o.loadStubs(ctx, true); err != nil {
		return err
	}

	if err := o.seedData(ctx); err != nil {
		return err
	}

	return nil
}

func (o *Orchestrator) startup(ctx context.Context) error {
	if err := o.checkDocker(ctx); err != nil {
		return err
	}

	// Clean log files from any previous session.
	devlogs.CleanLogDir(celerityDir(o.config.AppDir))

	if err := o.pullImage(ctx); err != nil {
		return err
	}

	if err := o.startDependencies(ctx); err != nil {
		return err
	}

	if err := o.seedConfig(ctx); err != nil {
		return err
	}

	// hostMode=false: stub URLs use Docker network addresses for the app container.
	if err := o.loadStubs(ctx, false); err != nil {
		return err
	}

	if err := o.seedData(ctx); err != nil {
		return err
	}

	if err := o.startContainer(ctx); err != nil {
		return err
	}

	return nil
}

func (o *Orchestrator) checkDocker(ctx context.Context) error {
	o.output.PrintProgress("Checking Docker...")
	if err := o.docker.CheckAvailability(ctx); err != nil {
		o.output.PrintError("Docker not available", err)
		return fmt.Errorf("docker check failed: %w", err)
	}
	o.output.PrintStep("Docker available")
	return nil
}

func (o *Orchestrator) pullImage(ctx context.Context) error {
	o.output.PrintProgress(fmt.Sprintf("Pulling image %s...", o.config.Image))
	progress := make(chan docker.ImagePullProgress, 100)

	errChan := make(chan error, 1)
	go func() {
		errChan <- o.docker.EnsureImage(ctx, o.config.Image, progress)
	}()

	var completed, total int
	seen := map[string]bool{}
	for event := range progress {
		if !seen[event.ID] && event.ID != "" {
			seen[event.ID] = true
			total += 1
		}
		if event.Status == "Pull complete" || event.Status == "Already exists" {
			completed++
			o.output.PrintProgress(fmt.Sprintf("Pulling: %d/%d layers complete", completed, total))
		}
	}

	if err := <-errChan; err != nil {
		o.output.PrintError("Image pull failed", err)
		return fmt.Errorf("image pull failed: %w", err)
	}

	o.output.PrintStep(fmt.Sprintf("Image %s ready", o.config.Image))
	return nil
}

func (o *Orchestrator) startDependencies(ctx context.Context) error {
	if o.compose == nil || !o.compose.HasServices() {
		return nil
	}

	o.output.PrintProgress("Starting dependencies...")
	if err := o.compose.Up(ctx, o.output.Writer()); err != nil {
		o.output.PrintError("Failed to start dependencies", err)
		o.printUnhealthyLogs(ctx)
		return fmt.Errorf("dependency startup failed: %w", err)
	}

	serviceNames := o.dependencyServiceNames()
	o.output.PrintStep(fmt.Sprintf("Dependencies ready (%s)", strings.Join(serviceNames, ", ")))
	return nil
}

func (o *Orchestrator) printUnhealthyLogs(ctx context.Context) {
	unhealthy := o.compose.UnhealthyServices(ctx)
	if len(unhealthy) == 0 {
		return
	}

	for _, svc := range unhealthy {
		logs, err := o.compose.Logs(ctx, svc, 30)
		if err != nil {
			o.logger.Debug("failed to fetch logs for unhealthy service", zap.String("service", svc), zap.Error(err))
			continue
		}
		if logs != "" {
			o.output.PrintInfo(fmt.Sprintf("Logs from unhealthy service %q:", svc))
			fmt.Fprintln(o.output.Writer(), logs)
		}
	}
}

func (o *Orchestrator) seedConfig(ctx context.Context) error {
	hostEndpoints := o.hostEndpoints()
	configEndpoint, ok := hostEndpoints[compose.EnvConfigEndpoint]
	if !ok {
		return nil
	}

	configs := seed.CollectConfigResources(o.config.Blueprint)
	if len(configs) == 0 {
		return nil
	}

	seeder, err := seed.NewValkeyConfigSeeder(configEndpoint, o.logger)
	if err != nil {
		o.output.PrintWarning("Config seeder init failed", err)
		return nil
	}
	defer seeder.Close()

	totalKeys := 0
	for _, cfg := range configs {
		values, err := seed.LoadAndMergeConfig(
			o.config.ConfigDir, o.config.SecretsDir, cfg.StoreName,
		)
		if err != nil {
			o.output.PrintWarning(fmt.Sprintf("Loading config for %s failed", cfg.StoreName), err)
			continue
		}
		if len(values) == 0 {
			continue
		}

		if err := seeder.SeedConfig(ctx, cfg.StoreName, values); err != nil {
			o.output.PrintWarning(fmt.Sprintf("Seeding config %s failed", cfg.StoreName), err)
			continue
		}
		totalKeys += len(values)
	}

	// Seed the "resources" config namespace with credentials for all
	// infrastructure resources (cache, datastore, bucket, queue, topic, SQL).
	resourceConfigValues := seed.ResourceConfigValues(o.config.Blueprint)
	if len(resourceConfigValues) > 0 {
		if err := seeder.SeedConfig(ctx, seed.ResourcesConfigStoreID, resourceConfigValues); err != nil {
			o.output.PrintWarning("Seeding resource credentials failed", err)
		} else {
			totalKeys += len(resourceConfigValues)
		}
	}

	if totalKeys > 0 {
		storeCount := len(configs)
		if len(resourceConfigValues) > 0 {
			storeCount++
		}
		o.output.PrintStep(fmt.Sprintf("Config seeded (%d keys across %d stores)", totalKeys, storeCount))
	}

	return nil
}

func (o *Orchestrator) seedData(ctx context.Context) error {
	hostEndpoints := o.hostEndpoints()
	if len(hostEndpoints) == 0 {
		return nil
	}

	provResult, err := o.provisionResources(ctx, hostEndpoints)
	if err != nil {
		o.output.PrintWarning("Provisioning failed", err)
		return nil
	}
	o.printProvisionSummary(provResult)

	// Apply SQL database schemas and migrations before seeding data.
	if err := o.applySqlSchemas(ctx, hostEndpoints); err != nil {
		o.output.PrintWarning("SQL schema application failed", err)
	}

	seedCfg, seedResult, err := o.loadAndExecuteSeed(ctx, hostEndpoints)
	if err != nil {
		o.output.PrintWarning("Seed data failed", err)
		return nil
	}
	o.printSeedSummary(seedResult)

	if seedCfg != nil && len(seedCfg.Hooks) > 0 {
		if err := o.runSeedHooks(ctx, seedCfg.Hooks, hostEndpoints); err != nil {
			o.output.PrintWarning("Seed hooks failed", err)
		}
	}

	return nil
}

func (o *Orchestrator) hostEndpoints() map[string]string {
	if o.config.ComposeCfg == nil {
		return nil
	}
	return o.config.ComposeCfg.HostEnvVars
}

func (o *Orchestrator) provisionResources(
	ctx context.Context,
	hostEndpoints map[string]string,
) (*seed.ProvisionResult, error) {
	datastoreProv, err := o.createDatastoreProvisioner(hostEndpoints)
	if err != nil {
		return nil, err
	}

	bucketProv, err := o.createBucketProvisioner(hostEndpoints)
	if err != nil {
		return nil, err
	}

	var streamEnabledTables map[string]bool
	if o.config.ComposeCfg != nil {
		streamEnabledTables = o.config.ComposeCfg.StreamEnabledTables
	}
	return seed.ProvisionFromBlueprint(
		ctx, o.config.Blueprint, datastoreProv, bucketProv,
		streamEnabledTables, o.logger,
	)
}

func (o *Orchestrator) createDatastoreProvisioner(
	hostEndpoints map[string]string,
) (seed.DatastoreProvisioner, error) {
	ep, ok := hostEndpoints[compose.EnvDatastoreEndpoint]
	if !ok {
		return nil, nil
	}

	switch o.config.DeployTarget {
	case compose.DeployTargetAWS, compose.DeployTargetAWSServerless:
		return seed.NewDynamoDBProvisioner(ep, o.logger), nil
	default:
		o.logger.Debug("no datastore provisioner for deploy target",
			zap.String("target", o.config.DeployTarget),
		)
		return nil, nil
	}
}

func (o *Orchestrator) createBucketProvisioner(
	hostEndpoints map[string]string,
) (seed.BucketProvisioner, error) {
	ep, ok := hostEndpoints[compose.EnvBucketEndpoint]
	if !ok {
		return nil, nil
	}

	accessKey := hostEndpoints[compose.EnvBucketAccessKey]
	secretKey := hostEndpoints[compose.EnvBucketSecretKey]
	minioEndpoint := strings.TrimPrefix(strings.TrimPrefix(ep, "https://"), "http://")
	return seed.NewMinIOProvisioner(minioEndpoint, accessKey, secretKey, o.logger)
}

func (o *Orchestrator) applySqlSchemas(
	ctx context.Context,
	hostEndpoints map[string]string,
) error {
	ep, ok := hostEndpoints[compose.EnvSqlDatabaseEndpoint]
	if !ok {
		return nil
	}

	dbResources := sqlschema.CollectDatabaseResources(o.config.Blueprint, o.config.AppDir)
	if len(dbResources) == 0 {
		return nil
	}

	engine := dbResources[0].Engine
	applier, err := sqlschema.NewApplier(engine, ep, o.logger)
	if err != nil {
		return fmt.Errorf("connecting to SQL database: %w", err)
	}
	defer applier.Close()

	for _, db := range dbResources {
		o.logger.Debug("applying SQL schema",
			zap.String("database", db.Name),
			zap.String("schemaPath", db.SchemaPath),
			zap.String("migrationsPath", db.MigrationsPath),
		)

		// Create the database if it doesn't exist (using admin connection).
		if err := applier.EnsureDatabase(ctx, db.Name); err != nil {
			return fmt.Errorf("ensuring database %s: %w", db.Name, err)
		}

		// Connect to the target database for schema + migration application.
		dbApplier, err := applier.ForDatabase(db.Name)
		if err != nil {
			return fmt.Errorf("connecting to database %s: %w", db.Name, err)
		}
		if err := dbApplier.ApplyAll(ctx, db.SchemaPath, db.MigrationsPath, db.Engine); err != nil {
			dbApplier.Close()
			return fmt.Errorf("applying schema for database %s: %w", db.Name, err)
		}
		dbApplier.Close()
	}

	o.output.PrintStep(fmt.Sprintf("SQL schemas applied (%d databases)", len(dbResources)))
	return nil
}

func (o *Orchestrator) loadAndExecuteSeed(
	ctx context.Context,
	hostEndpoints map[string]string,
) (*seed.SeedConfig, *seed.SeedResult, error) {
	if o.config.SeedDir == "" {
		return nil, &seed.SeedResult{}, nil
	}

	seedCfg, err := seed.LoadSeedConfig(o.config.SeedDir)
	if err != nil {
		return nil, nil, err
	}
	if seedCfg == nil {
		return nil, &seed.SeedResult{}, nil
	}

	nosqlSeeder, err := o.createNoSQLSeeder(hostEndpoints)
	if err != nil {
		return seedCfg, nil, err
	}

	storageUploader, err := o.createStorageUploader(hostEndpoints)
	if err != nil {
		return seedCfg, nil, err
	}

	sqlSeeder, err := o.createSQLSeeder(hostEndpoints)
	if err != nil {
		return seedCfg, nil, err
	}
	if sqlSeeder != nil {
		defer sqlSeeder.Close()
	}

	result, err := seed.ExecuteSeed(ctx, seedCfg, nosqlSeeder, storageUploader, sqlSeeder, o.logger)
	return seedCfg, result, err
}

func (o *Orchestrator) createNoSQLSeeder(
	hostEndpoints map[string]string,
) (seed.NoSQLSeeder, error) {
	ep, ok := hostEndpoints[compose.EnvDatastoreEndpoint]
	if !ok {
		return nil, nil
	}

	switch o.config.DeployTarget {
	case compose.DeployTargetAWS, compose.DeployTargetAWSServerless:
		return seed.NewDynamoDBSeeder(ep, o.logger), nil
	default:
		o.logger.Debug("no nosql seeder for deploy target",
			zap.String("target", o.config.DeployTarget),
		)
		return nil, nil
	}
}

func (o *Orchestrator) createStorageUploader(
	hostEndpoints map[string]string,
) (seed.StorageUploader, error) {
	ep, ok := hostEndpoints[compose.EnvBucketEndpoint]
	if !ok {
		return nil, nil
	}

	accessKey := hostEndpoints[compose.EnvBucketAccessKey]
	secretKey := hostEndpoints[compose.EnvBucketSecretKey]
	minioEndpoint := strings.TrimPrefix(strings.TrimPrefix(ep, "https://"), "http://")
	return seed.NewMinIOUploader(minioEndpoint, accessKey, secretKey, o.logger)
}

func (o *Orchestrator) createSQLSeeder(
	hostEndpoints map[string]string,
) (*sqlSeederAdapter, error) {
	ep, ok := hostEndpoints[compose.EnvSqlDatabaseEndpoint]
	if !ok {
		return nil, nil
	}

	dbResources := sqlschema.CollectDatabaseResources(o.config.Blueprint, o.config.AppDir)
	engine := "postgres"
	if len(dbResources) > 0 {
		engine = dbResources[0].Engine
	}

	applier, err := sqlschema.NewApplier(engine, ep, o.logger)
	if err != nil {
		return nil, fmt.Errorf("connecting to SQL database for seeding: %w", err)
	}

	return &sqlSeederAdapter{
		admin: applier,
		conns: make(map[string]*sqlschema.Applier),
	}, nil
}

// sqlSeederAdapter adapts sqlschema.Applier to the seed.SQLSeeder interface.
// It uses the admin Applier to derive per-database connections on demand,
// caching them for the lifetime of the seed run.
type sqlSeederAdapter struct {
	admin *sqlschema.Applier
	conns map[string]*sqlschema.Applier
}

func (a *sqlSeederAdapter) ExecSQL(ctx context.Context, databaseName string, sqlContent string) error {
	applier, err := a.applierFor(databaseName)
	if err != nil {
		return err
	}
	return applier.ExecSQL(ctx, sqlContent)
}

func (a *sqlSeederAdapter) applierFor(dbName string) (*sqlschema.Applier, error) {
	if existing, ok := a.conns[dbName]; ok {
		return existing, nil
	}
	dbApplier, err := a.admin.ForDatabase(dbName)
	if err != nil {
		return nil, fmt.Errorf("connecting to database %s for seeding: %w", dbName, err)
	}
	a.conns[dbName] = dbApplier
	return dbApplier, nil
}

func (a *sqlSeederAdapter) Close() error {
	var firstErr error
	for _, conn := range a.conns {
		if err := conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := a.admin.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (o *Orchestrator) runSeedHooks(
	ctx context.Context,
	scripts []string,
	hostEndpoints map[string]string,
) error {
	o.output.PrintInfo("Running seed hooks...")
	return seed.RunHooks(ctx, scripts, hostEndpoints, func(line seed.HookLine) {
		prefix := fmt.Sprintf("[hook:%s]", line.Script)
		if line.IsErr {
			o.output.PrintWarning(prefix, fmt.Errorf("%s", line.Line))
		} else {
			o.output.PrintInfo(fmt.Sprintf("%s %s", prefix, line.Line))
		}
	}, o.logger)
}

func (o *Orchestrator) printProvisionSummary(result *seed.ProvisionResult) {
	if result == nil {
		return
	}
	if len(result.Tables) == 0 && len(result.Buckets) == 0 {
		return
	}

	parts := []string{}
	if len(result.Tables) > 0 {
		parts = append(parts, fmt.Sprintf("tables: %s", strings.Join(result.Tables, ", ")))
	}
	if len(result.Buckets) > 0 {
		parts = append(parts, fmt.Sprintf("buckets: %s", strings.Join(result.Buckets, ", ")))
	}
	o.output.PrintStep(fmt.Sprintf("Provisioned %s", strings.Join(parts, " + ")))
}

func (o *Orchestrator) printSeedSummary(result *seed.SeedResult) {
	if result == nil || (result.Records == 0 && result.Files == 0 && result.SQLScripts == 0) {
		return
	}

	parts := []string{}
	if result.Records > 0 {
		parts = append(parts, fmt.Sprintf("%d records", result.Records))
	}
	if result.Files > 0 {
		parts = append(parts, fmt.Sprintf("%d files", result.Files))
	}
	if result.SQLScripts > 0 {
		parts = append(parts, fmt.Sprintf("%d sql scripts", result.SQLScripts))
	}
	o.output.PrintStep(fmt.Sprintf("Seeded %s", strings.Join(parts, " + ")))
}

func (o *Orchestrator) startContainer(ctx context.Context) error {
	o.output.PrintProgress("Starting container...")
	if err := o.docker.CleanupStale(ctx, o.config.ContainerCfg.ContainerName); err != nil {
		o.logger.Warn("stale container cleanup failed", zap.Error(err))
	}

	containerID, err := o.docker.CreateAndStart(ctx, o.config.ContainerCfg)
	if err != nil {
		o.output.PrintError("Failed to start container", err)
		return fmt.Errorf("container start failed: %w", err)
	}

	o.containerID = containerID
	o.output.PrintStep(fmt.Sprintf("Container started (%s)", o.config.ContainerCfg.ContainerName))
	return nil
}

func (o *Orchestrator) streamLogs(ctx context.Context) error {
	fileWriter, err := devlogs.NewLogFileWriter(celerityDir(o.config.AppDir))
	if err != nil {
		o.logger.Warn("log file setup failed", zap.Error(err))
	}
	if fileWriter != nil {
		defer fileWriter.Close()
		o.output.PrintInfo(fmt.Sprintf("Logs: %s", fileWriter.LogDir()))
	}

	streamer := devlogs.NewStreamer(o.docker, o.config.Runtime, o.output.isColor)
	streamer.FileWriter = fileWriter
	return streamer.Stream(ctx, o.containerID, devlogs.StreamOptions{
		Follow: true,
		Tail:   "all",
	}, o.output.writer)
}

func (o *Orchestrator) startWatcher(ctx context.Context) {
	if o.extractor == nil {
		return
	}

	watcher := NewHandlerWatcher(WatcherConfig{
		AppDir:      o.config.AppDir,
		Runtime:     o.config.Runtime,
		Extractor:   o.extractor,
		Blueprint:   o.config.Blueprint,
		SpecFormat:  o.config.SpecFormat,
		Docker:      o.docker,
		ContainerID: o.containerID,
		Output:      o.output,
		Logger:      o.logger,
	}, o.manifest)

	if err := watcher.Watch(ctx); err != nil {
		o.logger.Warn("file watcher stopped", zap.Error(err))
	}
}

func (o *Orchestrator) buildState(detached bool) *devstate.DevState {
	pid := os.Getpid()
	if detached {
		pid = 0
	}

	handlers := make([]devstate.HandlerSummary, len(o.config.HandlerInfos))
	for i, h := range o.config.HandlerInfos {
		handlers[i] = devstate.HandlerSummary{
			Name:   h.HandlerName,
			Type:   h.HandlerType,
			Method: h.Method,
			Path:   h.Path,
		}
	}

	composeProject := ""
	if o.config.ComposeCfg != nil {
		composeProject = o.config.ComposeCfg.ProjectName
	}

	return &devstate.DevState{
		ContainerID:    o.containerID,
		ContainerName:  o.config.ContainerCfg.ContainerName,
		ComposeProject: composeProject,
		Image:          o.config.Image,
		HostPort:       o.config.Port,
		AppDir:         o.config.AppDir,
		BlueprintFile:  o.config.MergedBlueprintPath,
		ServiceName:    o.config.ServiceName,
		Runtime:        o.config.Runtime,
		Handlers:       handlers,
		StartedAt:      time.Now(),
		PID:            pid,
		Detached:       detached,
	}
}

func (o *Orchestrator) dependencyServiceNames() []string {
	if o.config.ComposeCfg == nil {
		return nil
	}
	names := make([]string, 0, len(o.config.ComposeCfg.Services))
	for name := range o.config.ComposeCfg.Services {
		names = append(names, name)
	}
	return names
}

// HandleStaleState checks for a stale state file and cleans up if needed.
// Returns an error if an active dev environment is already running.
func HandleStaleState(
	ctx context.Context,
	appDir string,
	dockerMgr docker.RuntimeContainerManager,
	composeMgr *compose.ComposeManager,
	output *Output,
) error {
	state, err := devstate.Load(appDir)
	if err != nil {
		return fmt.Errorf("loading state file: %w", err)
	}
	if state == nil {
		return nil
	}

	if state.IsProcessAlive() {
		return fmt.Errorf(
			"dev environment already running (PID %d). Use 'celerity dev stop' first",
			state.PID,
		)
	}

	// For detached mode, check if container is still running.
	if state.Detached && state.ContainerID != "" {
		running, _ := dockerMgr.IsRunning(ctx, state.ContainerID)
		if running {
			return fmt.Errorf(
				"detached dev environment still running (container %s). Use 'celerity dev stop' first",
				state.ContainerName,
			)
		}
	}

	output.PrintInfo("Cleaning up stale dev environment...")
	if err := dockerMgr.CleanupStale(ctx, state.ContainerName); err != nil {
		output.PrintWarning("Stale container cleanup", err)
	}
	if composeMgr != nil && composeMgr.HasServices() {
		if err := composeMgr.Down(ctx); err != nil {
			output.PrintWarning("Stale compose cleanup", err)
		}
	}
	if err := devstate.Remove(appDir); err != nil {
		output.PrintWarning("State file removal", err)
	}
	output.PrintStep("Stale environment cleaned up")

	return nil
}

// StopFromState loads the state file and shuts down the environment.
// Used by `dev stop`.
func StopFromState(
	ctx context.Context,
	appDir string,
	dockerMgr docker.RuntimeContainerManager,
	composeMgr *compose.ComposeManager,
	output *Output,
	logger *zap.Logger,
) error {
	state, err := devstate.Load(appDir)
	if err != nil {
		return fmt.Errorf("loading state file: %w", err)
	}
	if state == nil {
		output.PrintNoEnvironment()
		return nil
	}

	output.PrintShutdownStarting()

	orch := &Orchestrator{
		config: OrchestratorConfig{
			AppDir:      appDir,
			ServiceName: state.ServiceName,
			ContainerCfg: &docker.ContainerConfig{
				ContainerName: state.ContainerName,
			},
		},
		docker:      dockerMgr,
		compose:     composeMgr,
		output:      output,
		logger:      logger,
		containerID: state.ContainerID,
	}

	return orch.Shutdown(ctx)
}

// LoadStateForCommand loads the state file and validates it for companion commands.
// Returns the state and an error if not found or stale.
func LoadStateForCommand(appDir string) (*devstate.DevState, error) {
	state, err := devstate.Load(appDir)
	if err != nil {
		return nil, fmt.Errorf("loading state file: %w", err)
	}
	if state == nil {
		return nil, fmt.Errorf("no dev environment running")
	}
	return state, nil
}

// celerityDir returns the .celerity directory path.
func celerityDir(appDir string) string {
	return filepath.Join(appDir, ".celerity")
}

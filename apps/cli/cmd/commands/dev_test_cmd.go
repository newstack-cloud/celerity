package commands

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/newstack-cloud/celerity/apps/cli/internal/compose"
	"github.com/newstack-cloud/celerity/apps/cli/internal/consts"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devconfig"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devrun"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devtest"
	"github.com/newstack-cloud/celerity/apps/cli/internal/docker"
	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
	"github.com/newstack-cloud/celerity/apps/cli/internal/seed"
	"github.com/newstack-cloud/deploy-cli-sdk/config"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/term"
)

func setupDevTestCommand(devCmd *cobra.Command, confProvider *config.Provider) {
	testCmd := &cobra.Command{
		Use:   "test",
		Short: "Run tests for a Celerity application",
		Long: `Run tests for a Celerity application with automatic infrastructure setup.

Infrastructure is started based on the test suites being run:
  --suite unit          No Docker infrastructure (tests use mocks/in-memory, default)
  --suite integration   Compose dependencies only (databases, caches, etc.)
  --suite api           Full stack: dependencies + app container

Multiple suites can be combined: --suite unit,integration

The test runner is auto-detected from your project files (package.json for
Node.js, pyproject.toml for Python). Use --test-command to override.

After tests complete, all infrastructure is torn down unless --no-teardown is set.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDevTest(cmd.Context(), confProvider)
		},
	}

	testCmd.Flags().StringP("blueprint-file", "b", "", "Blueprint file path (default: auto-detect)")
	confProvider.BindPFlag("devTestBlueprintFile", testCmd.Flags().Lookup("blueprint-file"))
	confProvider.BindEnvVar("devTestBlueprintFile", "CELERITY_CLI_DEV_TEST_BLUEPRINT_FILE")

	testCmd.Flags().StringP("deploy-config", "d", "", "Deploy config file path (default: auto-detect)")
	confProvider.BindPFlag("devTestDeployConfig", testCmd.Flags().Lookup("deploy-config"))
	confProvider.BindEnvVar("devTestDeployConfig", "CELERITY_CLI_DEV_TEST_DEPLOY_CONFIG")

	testCmd.Flags().StringP("port", "p", "8081", "Host port for the app container")
	confProvider.BindPFlag("devTestPort", testCmd.Flags().Lookup("port"))
	confProvider.BindEnvVar("devTestPort", "CELERITY_CLI_DEV_TEST_PORT")

	testCmd.Flags().String("app-dir", ".", "Application root directory")
	confProvider.BindPFlag("devTestAppDir", testCmd.Flags().Lookup("app-dir"))
	confProvider.BindEnvVar("devTestAppDir", "CELERITY_CLI_DEV_TEST_APP_DIR")

	testCmd.Flags().String("module-path", "", "Module entry point (auto-detected from blueprint runtime)")
	confProvider.BindPFlag("devTestModulePath", testCmd.Flags().Lookup("module-path"))
	confProvider.BindEnvVar("devTestModulePath", "CELERITY_CLI_DEV_TEST_MODULE_PATH")

	testCmd.Flags().String("image", "", "Override Docker image (skips GHCR lookup)")
	confProvider.BindPFlag("devTestImage", testCmd.Flags().Lookup("image"))
	confProvider.BindEnvVar("devTestImage", "CELERITY_CLI_DEV_TEST_IMAGE")

	testCmd.Flags().String("service-name", "", "Override service name (default: directory name)")
	confProvider.BindPFlag("devTestServiceName", testCmd.Flags().Lookup("service-name"))
	confProvider.BindEnvVar("devTestServiceName", "CELERITY_CLI_DEV_TEST_SERVICE_NAME")

	testCmd.Flags().BoolP("verbose", "v", false, "Enable debug logging in the runtime and SDK")
	confProvider.BindPFlag("devTestVerbose", testCmd.Flags().Lookup("verbose"))
	confProvider.BindEnvVar("devTestVerbose", "CELERITY_CLI_DEV_TEST_VERBOSE")

	testCmd.Flags().StringP("suite", "s", "unit", "Comma-separated test suites: unit, integration, api")
	confProvider.BindPFlag("devTestSuite", testCmd.Flags().Lookup("suite"))
	confProvider.BindEnvVar("devTestSuite", "CELERITY_CLI_DEV_TEST_SUITE")

	testCmd.Flags().Bool("coverage", true, "Enable coverage reporting")
	confProvider.BindPFlag("devTestCoverage", testCmd.Flags().Lookup("coverage"))
	confProvider.BindEnvVar("devTestCoverage", "CELERITY_CLI_DEV_TEST_COVERAGE")

	testCmd.Flags().String("test-command", "", "Override the auto-detected test command")
	confProvider.BindPFlag("devTestCommand", testCmd.Flags().Lookup("test-command"))
	confProvider.BindEnvVar("devTestCommand", "CELERITY_CLI_DEV_TEST_COMMAND")

	testCmd.Flags().String("health-timeout", "60s", "How long to wait for the app to be healthy")
	confProvider.BindPFlag("devTestHealthTimeout", testCmd.Flags().Lookup("health-timeout"))
	confProvider.BindEnvVar("devTestHealthTimeout", "CELERITY_CLI_DEV_TEST_HEALTH_TIMEOUT")

	testCmd.Flags().String("health-path", "", "Health check endpoint path (default: /runtime/health/check)")
	confProvider.BindPFlag("devTestHealthPath", testCmd.Flags().Lookup("health-path"))
	confProvider.BindEnvVar("devTestHealthPath", "CELERITY_CLI_DEV_TEST_HEALTH_PATH")

	testCmd.Flags().Bool("no-teardown", false, "Keep infrastructure running after tests complete")
	confProvider.BindPFlag("devTestNoTeardown", testCmd.Flags().Lookup("no-teardown"))
	confProvider.BindEnvVar("devTestNoTeardown", "CELERITY_CLI_DEV_TEST_NO_TEARDOWN")

	testCmd.Flags().Bool("no-local-auth", false, "Do not override the blueprint JWT issuer with the local dev auth server")
	confProvider.BindPFlag("devTestNoLocalAuth", testCmd.Flags().Lookup("no-local-auth"))
	confProvider.BindEnvVar("devTestNoLocalAuth", "CELERITY_CLI_DEV_TEST_NO_LOCAL_AUTH")

	testCmd.Flags().Bool("tracing", false, "Run a trace collector in the session and export the application's spans to it")
	confProvider.BindPFlag("devTestTracing", testCmd.Flags().Lookup("tracing"))
	confProvider.BindEnvVar("devTestTracing", "CELERITY_CLI_DEV_TEST_TRACING")

	devCmd.AddCommand(testCmd)
}

type testFlags struct {
	suites                []devtest.TestSuite
	infraLevel            devtest.InfraLevel
	coverage              bool
	testCommand           string
	noTeardown            bool
	healthTimeout         time.Duration
	healthTimeoutExplicit bool
	healthPath            string
}

func parseTestFlags(confProvider *config.Provider) (*testFlags, error) {
	suiteStr, _ := confProvider.GetString("devTestSuite")
	suites, err := parseSuites(suiteStr)
	if err != nil {
		return nil, err
	}

	healthTimeoutStr, _ := confProvider.GetString("devTestHealthTimeout")
	healthTimeoutExplicit := healthTimeoutStr != "" && healthTimeoutStr != "60s"
	healthTimeout, err := time.ParseDuration(healthTimeoutStr)
	if err != nil {
		healthTimeout = 60 * time.Second
	}

	coverage, _ := confProvider.GetBool("devTestCoverage")
	testCommand, _ := confProvider.GetString("devTestCommand")
	noTeardown, _ := confProvider.GetBool("devTestNoTeardown")
	healthPath, _ := confProvider.GetString("devTestHealthPath")

	return &testFlags{
		suites:                suites,
		infraLevel:            devtest.InfraLevelForSuites(suites),
		coverage:              coverage,
		testCommand:           testCommand,
		noTeardown:            noTeardown,
		healthTimeout:         healthTimeout,
		healthTimeoutExplicit: healthTimeoutExplicit,
		healthPath:            healthPath,
	}, nil
}

// Resolves config, starts Docker dependencies and, for API suites, the app
// container, then waits for health. A nil orchestrator is returned when no
// infrastructure is needed, as for unit-only suites.
func setupTestInfra(
	ctx context.Context,
	opts devconfig.ResolveOpts,
	flags *testFlags,
	output *devrun.Output,
	logger *zap.Logger,
) (*devrun.Orchestrator, *devconfig.ResolvedConfig, error) {
	resolved, err := devconfig.Resolve(ctx, opts, logger)
	if err != nil {
		output.PrintError("Config resolution failed", err)
		return nil, nil, err
	}

	// Python projects install dependencies on container startup,
	// so the default health timeout needs to be longer.
	if !flags.healthTimeoutExplicit && resolved.Runtime == consts.LanguagePython {
		flags.healthTimeout = 3 * time.Minute
	}

	dockerMgr, err := docker.NewRuntimeContainer(logger)
	if err != nil {
		return nil, nil, err
	}

	composeMgr, err := compose.NewComposeManager(
		resolved.ComposeConfig.ProjectName, resolved.AppDir, logger,
	)
	if err != nil {
		return nil, nil, err
	}

	if err := devrun.HandleStaleState(ctx, resolved.AppDir, dockerMgr, composeMgr, output); err != nil {
		return nil, nil, err
	}

	var extractor *preprocess.Extractor
	conv, _ := consts.ConventionsForRuntime(resolved.Runtime)
	if conv.SupportsExtraction {
		extractor = preprocess.NewExtractor(preprocess.ExtractorConfig{
			Runtime:     resolved.Runtime,
			ModulePath:  resolved.ModulePath,
			ProjectRoot: resolved.AppDir,
		}, logger)
	}

	orch := devrun.NewOrchestrator(
		devrun.OrchestratorConfig{
			AppDir:              resolved.AppDir,
			Port:                resolved.Port,
			ServiceName:         resolved.ServiceName,
			Image:               resolved.RuntimeImage,
			DeployTarget:        resolved.DeployTarget,
			Runtime:             resolved.Runtime,
			MergedBlueprintPath: resolved.MergedBlueprintPath,
			ModulePath:          resolved.ModulePath,
			SeedDir:             resolved.SeedDir,
			ConfigDir:           resolved.ConfigDir,
			SecretsDir:          resolved.SecretsDir,
			Blueprint:           resolved.Blueprint,
			SpecFormat:          resolved.SpecFormat,
			HandlerInfos:        resolved.HandlerInfos,
			Manifest:            resolved.Manifest,
			ContainerCfg:        resolved.ContainerConfig,
			ComposeCfg:          resolved.ComposeConfig,
		},
		dockerMgr,
		composeMgr,
		extractor,
		output,
		logger,
	)

	switch flags.infraLevel {
	case devtest.InfraLevelFull:
		if err := orch.StartFull(ctx); err != nil {
			return orch, resolved, err
		}

		healthURL := fmt.Sprintf("http://localhost:%s", resolved.Port)
		if err := devtest.WaitForHealth(ctx, healthURL, flags.healthPath, flags.healthTimeout, output); err != nil {
			output.PrintError("Health check failed", err)
			return orch, resolved, err
		}

	case devtest.InfraLevelCompose:
		if err := orch.StartInfraOnly(ctx); err != nil {
			return orch, resolved, err
		}
	}

	return orch, resolved, nil
}

// Assembles the environment variables passed to the test process, covering host
// endpoints, config store IDs and Valkey connection overrides.
func buildTestEnv(resolved *devconfig.ResolvedConfig) map[string]string {
	testEnv := map[string]string{}
	if resolved == nil {
		return testEnv
	}

	if resolved.ComposeConfig != nil {
		maps.Copy(testEnv, resolved.ComposeConfig.HostEnvVars)
	}

	if resolved.Blueprint == nil {
		return testEnv
	}

	// Add config store ID env vars and Valkey connection so the SDK's
	// testing utilities can resolve config namespaces from Valkey.
	maps.Copy(testEnv, seed.ConfigStoreIDEnvVars(resolved.Blueprint))
	maps.Copy(testEnv, seed.ResourcesConfigStoreEnvVars())

	// The runtime container uses the Docker hostname "valkey" but tests
	// run on the host and need localhost with the offset port.
	testEnv["CELERITY_CONFIG_VALKEY_HOST"] = "localhost"
	// Valkey port (6379) is in HostEnvVars as part of the Redis URL;
	// extract the port from CELERITY_REDIS_ENDPOINT if available.
	if redisURL, ok := testEnv["CELERITY_REDIS_ENDPOINT"]; ok {
		if u, err := url.Parse(redisURL); err == nil && u.Port() != "" {
			testEnv["CELERITY_CONFIG_VALKEY_PORT"] = u.Port()
		}
	}

	return testEnv
}

func runDevTest(ctx context.Context, confProvider *config.Provider) error {
	logger, logHandle, err := setupDevLogger()
	if err != nil {
		return err
	}
	defer logHandle.Close()

	isColor := term.IsTerminal(int(os.Stdout.Fd()))
	output := devrun.NewOutput(os.Stdout, isColor)

	flags, err := parseTestFlags(confProvider)
	if err != nil {
		output.PrintError("Invalid --suite value", err)
		return err
	}

	opts := testResolveOpts(confProvider)

	var orch *devrun.Orchestrator
	var resolved *devconfig.ResolvedConfig

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	testsFailed := false

	teardown := func() {
		if orch == nil || flags.noTeardown {
			return
		}
		// Logs are only dumped when tests failed or errored, since on success
		// they are noise.
		if testsFailed && flags.infraLevel >= devtest.InfraLevelCompose {
			if logDir := orch.DumpLogs(context.Background()); logDir != "" {
				output.PrintStep(fmt.Sprintf("Logs saved to %s", logDir))
			}
		}
		output.PrintInfo("Stopping test environment...")
		if err := orch.Shutdown(context.Background()); err != nil {
			output.PrintWarning("Teardown failed", err)
		}
	}

	if flags.infraLevel > devtest.InfraLevelNone {
		orch, resolved, err = setupTestInfra(ctx, opts, flags, output, logger)
		if err != nil {
			teardown()
			return err
		}
	}

	// A unit-only run resolves no config, so the runtime for the test runner
	// comes from the project files instead.
	runtime := ""
	appDir := opts.AppDir
	if appDir == "" || appDir == "." {
		appDir, _ = os.Getwd()
	}
	if resolved != nil {
		runtime = resolved.Runtime
		appDir = resolved.AppDir
	} else {
		detectedRuntime, err := detectRuntimeFromProject(appDir)
		if err != nil {
			return fmt.Errorf("cannot detect runtime for test runner: %w; use --test-command to specify the test command", err)
		}
		runtime = detectedRuntime
	}

	toolchain, err := testToolchain(resolved, runtime, appDir)
	if err != nil {
		teardown()
		return err
	}

	runner, err := devtest.RunnerForRuntime(runtime, toolchain, logger)
	if err != nil {
		teardown()
		return err
	}

	testEnv := buildTestEnv(resolved)

	suiteLabels := make([]string, len(flags.suites))
	for i, s := range flags.suites {
		suiteLabels[i] = string(s)
	}
	output.PrintTestHeader(suiteLabels)

	runConfig := devtest.RunConfig{
		AppDir:      appDir,
		Runtime:     runtime,
		Suites:      flags.suites,
		Coverage:    flags.coverage,
		TestCommand: flags.testCommand,
		Verbose:     opts.Verbose,
		Env:         testEnv,
	}
	if flags.infraLevel == devtest.InfraLevelFull && resolved != nil {
		runConfig.HostPort = resolved.Port
	}

	resultChan := make(chan *devtest.RunResult, 1)
	errChan := make(chan error, 1)
	go func() {
		result, err := runner.Run(ctx, runConfig)
		if err != nil {
			errChan <- err
		} else {
			resultChan <- result
		}
	}()

	var result *devtest.RunResult
	select {
	case <-sigChan:
		cancel()
		output.PrintInfo("\nInterrupted")
		testsFailed = true
		teardown()
		os.Exit(130)
	case err := <-errChan:
		output.PrintError("Test runner failed", err)
		testsFailed = true
		teardown()
		return err
	case result = <-resultChan:
	}

	if result.ExitCode == 0 {
		output.PrintTestPassed()
	} else {
		output.PrintTestFailed(result.ExitCode)
		testsFailed = true
	}

	if result.CoveragePath != "" {
		output.PrintInfo(fmt.Sprintf("Coverage report: %s", result.CoveragePath))
	}

	teardown()

	if result.ExitCode != 0 {
		os.Exit(result.ExitCode)
	}

	return nil
}

func testResolveOpts(confProvider *config.Provider) devconfig.ResolveOpts {
	blueprintFile, _ := confProvider.GetString("devTestBlueprintFile")
	deployConfig, _ := confProvider.GetString("devTestDeployConfig")
	port, _ := confProvider.GetString("devTestPort")
	appDir, _ := confProvider.GetString("devTestAppDir")
	modulePath, _ := confProvider.GetString("devTestModulePath")
	image, _ := confProvider.GetString("devTestImage")
	serviceName, _ := confProvider.GetString("devTestServiceName")
	verbose, _ := confProvider.GetBool("devTestVerbose")
	noLocalAuth, _ := confProvider.GetBool("devTestNoLocalAuth")
	tracing, _ := confProvider.GetBool("devTestTracing")

	return devconfig.ResolveOpts{
		BlueprintFile: blueprintFile,
		DeployConfig:  deployConfig,
		Port:          port,
		AppDir:        appDir,
		ModulePath:    modulePath,
		Image:         image,
		ServiceName:   serviceName,
		Mode:          "test",
		Verbose:       verbose,
		LocalAuth:     !noLocalAuth,
		Tracing:       tracing,
	}
}

func parseSuites(raw string) ([]devtest.TestSuite, error) {
	parts := strings.Split(raw, ",")
	var suites []devtest.TestSuite
	for _, p := range parts {
		s := devtest.TestSuite(strings.TrimSpace(p))
		if !isValidSuite(s) {
			return nil, fmt.Errorf(
				"unknown suite %q; valid values: unit, integration, api", s,
			)
		}
		suites = append(suites, s)
	}
	if len(suites) == 0 {
		return []devtest.TestSuite{devtest.SuiteUnit}, nil
	}
	return suites, nil
}

func isValidSuite(s devtest.TestSuite) bool {
	return slices.Contains(devtest.ValidSuites, s)
}

// Used in unit-only mode, where no blueprint resolution is performed.
func detectRuntimeFromProject(appDir string) (string, error) {
	if _, err := os.Stat(appDir + "/package.json"); err == nil {
		return "nodejs", nil
	}
	if _, err := os.Stat(appDir + "/pyproject.toml"); err == nil {
		return "python", nil
	}
	if _, err := os.Stat(appDir + "/go.mod"); err == nil {
		// The OS-only runtime identifier rather than "go", because a Go
		// application runs on one and that is what the runner registry and the
		// conventions are keyed by.
		return consts.RuntimeOSOnlyDebian13, nil
	}
	return "", fmt.Errorf("no package.json, pyproject.toml, or go.mod found in %s", appDir)
}

// Finds the build tooling whose test command a run should use.
//
// Taken from the resolved config where there is one, so the test run uses the
// same toolchain the application was resolved with. A unit-only run resolves no
// config at all, so one is worked out from the project directory instead.
func testToolchain(
	resolved *devconfig.ResolvedConfig,
	runtime string,
	appDir string,
) (handlertoolchain.Toolchain, error) {
	if resolved != nil {
		return resolved.Toolchain, nil
	}
	if !handlertoolchain.IsOSOnly(runtime) {
		return nil, nil
	}

	var handlerBuild *handlertoolchain.Config
	if path := devconfig.FindDeployConfig(appDir); path != "" {
		var err error
		handlerBuild, err = devconfig.ReadHandlerBuild(path)
		if err != nil {
			return nil, err
		}
	}

	return handlertoolchain.Resolve(handlertoolchain.Options{
		Runtime: runtime,
		AppDir:  appDir,
		Config:  handlerBuild,
	})
}

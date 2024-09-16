package main

import (
	"context"
	"log"
	"os"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/two-hundred/celerity/libs/blueprint/container"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
	"github.com/two-hundred/celerity/tools/blueprint-ls/internal/blueprint"
	"github.com/two-hundred/celerity/tools/blueprint-ls/internal/languageserver"
	"github.com/two-hundred/celerity/tools/blueprint-ls/internal/languageservices"
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
	"github.com/two-hundred/ls-builder/server"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	logger, logFile, err := setupLogger()
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	state := languageservices.NewState()
	settingsService := languageservices.NewSettingsService(
		state,
		languageserver.ConfigSection,
		logger,
	)
	traceService := lsp.NewTraceService(logger)

	providers, err := blueprint.LoadProviders(context.TODO())
	if err != nil {
		log.Fatal(err)
	}

	transformers, err := blueprint.LoadTransformers(context.TODO())
	if err != nil {
		log.Fatal(err)
	}

	functionRegistry := provider.NewFunctionRegistry(providers)
	resourceRegistry := resourcehelpers.NewRegistry(providers, transformers)
	dataSourceRegistry := provider.NewDataSourceRegistry(providers)
	customVarTypeRegistry := provider.NewCustomVariableTypeRegistry(providers)

	completionService := languageservices.NewCompletionService(
		resourceRegistry,
		dataSourceRegistry,
		customVarTypeRegistry,
		functionRegistry,
		state,
		logger,
	)

	blueprintLoader := container.NewDefaultLoader(
		providers,
		map[string]transform.SpecTransformer{},
		/* stateContainer */ nil,
		/* updateChan */ nil,
		// TODO: instantiate ref chain collector on each load!!!
		validation.NewRefChainCollector(),
		// Disable runtime value validation as it is not needed for diagnostics.
		container.WithLoaderValidateRuntimeValues(false),
		// Disable spec transformation as it is not needed for diagnostics.
		container.WithLoaderTransformSpec(false),
	)

	diagnosticErrorService := languageservices.NewDiagnosticErrorService(state, logger)
	diagnosticService := languageservices.NewDiagnosticsService(
		state,
		settingsService,
		diagnosticErrorService,
		blueprintLoader,
		logger,
	)

	signatureService := languageservices.NewSignatureService(
		functionRegistry,
		logger,
	)
	hoverService := languageservices.NewHoverService(
		functionRegistry,
		resourceRegistry,
		dataSourceRegistry,
		signatureService,
		logger,
	)
	symbolService := languageservices.NewSymbolService(
		state,
		logger,
	)

	app := languageserver.NewApplication(
		state,
		settingsService,
		traceService,
		functionRegistry,
		resourceRegistry,
		dataSourceRegistry,
		blueprintLoader,
		completionService,
		diagnosticService,
		signatureService,
		hoverService,
		symbolService,
		logger,
	)
	app.Setup()

	srv := server.NewServer(app.Handler(), true, logger, nil)

	stdio := server.Stdio{}
	conn := server.NewStreamConnection(
		// Wrapping in async handler is essential to avoid a deadlock
		// when the server sends a request to the client while it is handling
		// a request from the client.
		// For example, when handling the hover request, the server may fetch
		// configuration settings from the client, without an async handler, this will
		// block until the configured timeout is reached and the context is cancelled.
		jsonrpc2.AsyncHandler(srv.NewHandler()),
		stdio,
	)
	srv.Serve(conn, logger)
}

func setupLogger() (*zap.Logger, *os.File, error) {
	logFileHandle, err := os.OpenFile("blueprint-ls.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, err
	}
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder

	writerSync := zapcore.NewMultiWriteSyncer(
		// stdout and stdin are used for communication with the client
		// and should not be logged to.
		zapcore.AddSync(os.Stderr),
		zapcore.AddSync(logFileHandle),
	)
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		writerSync,
		zap.DebugLevel,
	)
	logger := zap.New(core)
	return logger, logFileHandle, nil
}

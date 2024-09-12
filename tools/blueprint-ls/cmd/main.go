package main

import (
	"context"
	"log"
	"os"

	"github.com/two-hundred/celerity/libs/blueprint/container"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
	"github.com/two-hundred/celerity/tools/blueprint-ls/internal/blueprint"
	"github.com/two-hundred/celerity/tools/blueprint-ls/internal/languageserver"
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

	state := languageserver.NewState()
	traceService := lsp.NewTraceService(logger)

	providers, err := blueprint.LoadProviders(context.TODO())
	if err != nil {
		log.Fatal(err)
	}

	transformers, err := blueprint.LoadTransformers(context.TODO())
	if err != nil {
		log.Fatal(err)
	}

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

	app := languageserver.NewApplication(
		state,
		traceService,
		providers,
		transformers,
		blueprintLoader,
		logger,
	)
	app.Setup()

	srv := server.NewServer(app.Handler(), true, logger, nil)

	stdio := server.Stdio{}
	conn := server.NewStreamConnection(srv.NewHandler(), stdio)
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

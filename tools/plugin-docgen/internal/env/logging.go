package env

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// CreateLogger creates a logger for tool logs
// (debug, info, error etc.) that writes to the provided
// stdout and stderr targets.
// This will determine the log level based on the
// provided configuration.
func CreateLogger(
	stdoutTarget zapcore.WriteSyncer,
	stderrTarget zapcore.WriteSyncer,
	config *Config,
) (*zap.Logger, error) {

	zapLevel, err := zapcore.ParseLevel(config.LogLevel)
	if err != nil {
		return nil, err
	}

	zapConf := zap.NewDevelopmentEncoderConfig()
	consoleErrors := zapcore.Lock(stderrTarget)
	consoleDebugging := zapcore.Lock(stdoutTarget)

	consoleEncoder := zapcore.NewConsoleEncoder(zapConf)
	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, consoleDebugging, zapLevel),
		// zapLevel is expected to be switched between debug and info,
		// so hardcoding the error level threshold shouldn't be a problem.
		// If this is also set to zapLevel, it will produce duplicate logs.
		zapcore.NewCore(consoleEncoder, consoleErrors, zapcore.ErrorLevel),
	)
	return zap.New(core), nil
}

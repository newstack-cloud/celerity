package env

import "github.com/caarlos0/env/v11"

// Config holds the configuration for the github
// registry service.
type Config struct {
	PluginPath           string `env:"CELERITY_DEPLOY_ENGINE_PLUGIN_PATH"`
	PluginLogFileRootDir string `env:"CELERITY_PLUGIN_DOCGEN_PLUGIN_LOG_FILE_ROOT_DIR"`
	LogLevel             string `env:"CELERITY_PLUGIN_DOCGEN_LOG_LEVEL" envDefault:"info"`
	LaunchWaitTimeoutMS  int    `env:"CELERITY_PLUGIN_DOCGEN_PLUGIN_LAUNCH_WAIT_TIMEOUT_MS" envDefault:"10000"`
	GenerateTimeoutMS    int    `env:"CELERITY_PLUGIN_DOCGEN_GENERATE_TIMEOUT_MS" envDefault:"30000"`
}

// LoadConfig loads environment configuration
// for the plugin JSON doc generator tool.
func LoadConfig() (Config, error) {
	return env.ParseAs[Config]()
}

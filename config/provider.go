package config

import (
	"github.com/glennprays/log"
)

// ProvideConfig loads application configuration
func ProvideConfig() (*Config, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	cfg.AppVersion = AppVersion
	return cfg, nil
}

// ProvideLogger initializes logger based on configuration
func ProvideLogger(cfg *Config) (*log.Logger, error) {
	// Map string level to log.Level
	var level log.Level
	switch cfg.LogLevel {
	case "debug":
		level = log.DebugLevel
	case "info":
		level = log.InfoLevel
	case "warn":
		level = log.WarnLevel
	case "error":
		level = log.ErrorLevel
	case "fatal":
		level = log.FatalLevel
	default:
		level = log.InfoLevel
	}

	// Map string output to log.OutputType
	var output log.OutputType
	if cfg.LogOutput == "file" {
		output = log.OutputFile
	} else {
		output = log.OutputStdout
	}

	logConfig := log.Config{
		Service:      "whatsapp-gateway",
		Env:          cfg.Env.String(),
		Level:        level,
		Output:       output,
		FilePath:     cfg.LogFilePath,
		EnableCaller: cfg.EnableCaller,
	}

	return log.New(logConfig)
}

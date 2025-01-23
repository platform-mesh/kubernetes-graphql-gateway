package config

import (
	"github.com/vrischmann/envconfig"
)

type Config struct {
	Port       string `envconfig:"default=8080,optional"`
	LogLevel   string `envconfig:"default=INFO,optional"`
	WatchedDir string `envconfig:"default=definitions,required"`
	HandlerCfg HandlerConfig
}

type HandlerConfig struct {
	Pretty     bool `envconfig:"default=true,optional"`
	Playground bool `envconfig:"default=true,optional"`
	GraphiQL   bool `envconfig:"default=true,optional"`
}

// NewFromEnv creates a Config from environment values
func NewFromEnv() (Config, error) {
	appConfig := Config{}
	err := envconfig.Init(&appConfig)
	return appConfig, err
}

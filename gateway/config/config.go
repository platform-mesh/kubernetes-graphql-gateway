package config

import (
	"github.com/vrischmann/envconfig"
)

type Config struct {
	// common with listener
	OpenApiDefinitionsPath string `envconfig:"default=./bin/definitions"`
	EnableKcp              bool   `envconfig:"default=true,optional"`

	// for gateway
	Port             string `envconfig:"default=8080,optional"`
	LogLevel         string `envconfig:"default=INFO,optional"`
	LocalDevelopment bool   `envconfig:"default=false,optional"`
	HandlerCfg       HandlerConfig
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

package config

import (
	"github.com/vrischmann/envconfig"
)

type Config struct {
	Listener
	Gateway

	OpenApiDefinitionsPath string `envconfig:"default=./bin/definitions"`
	EnableKcp              bool   `envconfig:"default=true,optional"`
	LocalDevelopment       bool   `envconfig:"default=false,optional"`
}

type Gateway struct {
	Port              string `envconfig:"default=8080,optional"`
	LogLevel          string `envconfig:"default=INFO,optional"`
	UserNameClaim     string `envconfig:"default=email,optional"`
	ShouldImpersonate bool   `envconfig:"default=true,optional"`

	HandlerCfg struct {
		Pretty     bool `envconfig:"default=true,optional"`
		Playground bool `envconfig:"default=true,optional"`
		GraphiQL   bool `envconfig:"default=true,optional"`
	}

	Cors struct {
		Enabled        bool     `envconfig:"default=false,optional"`
		AllowedOrigins []string `envconfig:"default=*,optional"`
		AllowedHeaders []string `envconfig:"default=*,optional"`
	}
}

type Listener struct {
	MetricsAddr          string `envconfig:"default=0,optional"`
	EnableLeaderElection bool   `envconfig:"default=false,optional"`
	ProbeAddr            string `envconfig:"default=:8081,optional"`
	SecureMetrics        bool   `envconfig:"default=true,optional"`
	EnableHTTP2          bool   `envconfig:"default=false,optional"`
}

// NewFromEnv creates a Gateway from environment values
func NewFromEnv() (*Config, error) {
	cfg := &Config{}
	err := envconfig.Init(cfg)
	return cfg, err
}

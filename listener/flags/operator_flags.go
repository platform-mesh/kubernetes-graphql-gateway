package flags

import (
	"github.com/vrischmann/envconfig"
)

type Flags struct {
	// common with gateway
	OpenApiDefinitionsPath string `envconfig:"default=./bin/definitions"`
	EnableKcp              bool   `envconfig:"default=true,optional"`

	// for listener
	MetricsAddr          string `envconfig:"default=0,optional"`
	EnableLeaderElection bool   `envconfig:"default=false,optional"`
	ProbeAddr            string `envconfig:"default=:8081,optional"`
	SecureMetrics        bool   `envconfig:"default=true,optional"`
	EnableHTTP2          bool   `envconfig:"default=false,optional"`
}

func NewFromEnv() (*Flags, error) {
	opFlags := &Flags{}
	err := envconfig.Init(opFlags)

	return opFlags, err
}

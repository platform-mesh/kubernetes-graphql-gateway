package gateway

import (
	"fmt"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/http"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/options"
)

type Config struct {
	Options *options.CompletedOptions

	Gateway    *gateway.Service
	HTTPServer *http.Server
}

func NewConfig(options *options.CompletedOptions) (*Config, error) {
	config := &Config{
		Options: options,
	}
	gatewayServer, err := gateway.New(gateway.GatewayConfig{
		DevelopmentDisableAuth: config.Options.DevelopmentDisableAuth,
		GraphQLPretty:          true, // Always pretty print for readability
		GraphQLPlayground:      config.Options.PlaygroundEnabled,
		GraphQLGraphiQL:        config.Options.PlaygroundEnabled,
		SchemaDirectory:        config.Options.SchemasDir,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway server: %w", err)
	}
	config.Gateway = gatewayServer

	httpServer, err := http.NewServer(http.ServerConfig{
		Gateway: gatewayServer,
		Addr:    fmt.Sprintf("%s:%d", config.Options.ServerBindAddress, config.Options.ServerBindPort),
		CORSConfig: http.CORSConfig{
			AllowedOrigins:   config.Options.CORSAllowedOrigins,
			AllowedHeaders:   config.Options.CORSAllowedHeaders,
			AllowCredentials: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP server: %w", err)
	}

	config.HTTPServer = httpServer

	return config, nil
}

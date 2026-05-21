package gateway

import (
	"fmt"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway"
	gatewayconfig "github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/config"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/metrics"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/middleware"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/http"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/options"
	"github.com/prometheus/client_golang/prometheus"
)

type Config struct {
	Options *options.CompletedOptions

	Gateway    *gateway.Service
	HTTPServer *http.Server
}

func NewConfig(opts *options.CompletedOptions) (*Config, error) {
	cfg := &Config{
		Options: opts,
	}

	gatewayServer, err := gateway.New(gatewayconfig.Gateway{
		SchemaHandler:      cfg.Options.SchemaHandler,
		SchemaDirectory:    cfg.Options.SchemasDir,
		GRPCAddress:        cfg.Options.GRPCListenerAddress,
		GRPCMaxRecvMsgSize: cfg.Options.GRPCMaxRecvMsgSize,
		GraphQL: gatewayconfig.GraphQL{
			Pretty:            true,
			PlaygroundEnabled: cfg.Options.PlaygroundEnabled,
			GraphiQL:          cfg.Options.PlaygroundEnabled,
		},
		Limits: gatewayconfig.Limits{
			MaxQueryDepth:      cfg.Options.MaxQueryDepth,
			MaxQueryComplexity: cfg.Options.MaxQueryComplexity,
			MaxQueryBatchSize:  cfg.Options.MaxQueryBatchSize,
		},
		TokenReviewCacheTTL: cfg.Options.TokenReviewCacheTTL,
		Metrics:             metrics.NewCollector(prometheus.DefaultRegisterer),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway server: %w", err)
	}
	cfg.Gateway = gatewayServer

	subMetrics := metrics.NewSubscriptionMetrics(prometheus.DefaultRegisterer)

	httpServer, err := http.NewServer(http.ServerConfig{
		Gateway:                  gatewayServer,
		ReadyzCheck:              gatewayServer.IsReady,
		Addr:                     fmt.Sprintf("%s:%d", cfg.Options.ServerBindAddress, cfg.Options.ServerBindPort),
		PlaygroundEnabled:        cfg.Options.PlaygroundEnabled,
		MaxRequestBodyBytes:      cfg.Options.MaxRequestBodyBytes,
		MaxInFlightRequests:      cfg.Options.MaxInFlightRequests,
		MaxInFlightSubscriptions: cfg.Options.MaxInFlightSubscriptions,
		RequestTimeout:           cfg.Options.RequestTimeout,
		SubscriptionTimeout:      cfg.Options.SubscriptionTimeout,
		ReadHeaderTimeout:        cfg.Options.ReadHeaderTimeout,
		IdleTimeout:              cfg.Options.IdleTimeout,
		EndpointSuffix:           cfg.Options.EndpointSuffix,
		SubscriptionMetrics: &middleware.InFlightMetrics{
			Active:   subMetrics.Active,
			Total:    subMetrics.Total,
			Rejected: subMetrics.Rejected,
		},
		CORSConfig: http.CORSConfig{
			AllowedOrigins:   cfg.Options.CORSAllowedOrigins,
			AllowedHeaders:   cfg.Options.CORSAllowedHeaders,
			AllowCredentials: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP server: %w", err)
	}

	cfg.HTTPServer = httpServer

	return cfg, nil
}

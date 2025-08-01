package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	openmfpcontext "github.com/openmfp/golang-commons/context"
	"github.com/openmfp/golang-commons/sentry"
	"github.com/openmfp/golang-commons/traces"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/openmfp/golang-commons/logger"

	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager"
)

var gatewayCmd = &cobra.Command{
	Use:     "gateway",
	Short:   "Run the GQL Gateway",
	Example: "go run main.go gateway",
	RunE: func(_ *cobra.Command, _ []string) error {
		log, err := setupLogger(defaultCfg.Log.Level)
		if err != nil {
			return fmt.Errorf("failed to setup logger: %w", err)
		}

		log.Info().Str("LogLevel", log.GetLevel().String()).Msg("Starting gateway server...")

		ctx, _, shutdown := openmfpcontext.StartContext(log, appCfg, 1*time.Second)
		defer shutdown()

		if err := initializeSentry(ctx, log); err != nil {
			return err
		}

		ctrl.SetLogger(log.Logr())

		gatewayInstance, err := manager.NewGateway(ctx, log, appCfg)
		if err != nil {
			log.Error().Err(err).Msg("Error creating gateway")
			return fmt.Errorf("failed to create gateway: %w", err)
		}

		tracingShutdown, err := initializeTracing(ctx, log)
		if err != nil {
			return err
		}
		defer func() {
			if err := tracingShutdown(ctx); err != nil {
				log.Error().Err(err).Msg("failed to shutdown TracerProvider")
			}
		}()

		return runServers(ctx, log, gatewayInstance)
	},
}

func initializeSentry(ctx context.Context, log *logger.Logger) error {
	if defaultCfg.Sentry.Dsn == "" {
		return nil
	}

	err := sentry.Start(ctx,
		defaultCfg.Sentry.Dsn, defaultCfg.Environment, defaultCfg.Region,
		defaultCfg.Image.Name, defaultCfg.Image.Tag,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Sentry init failed")
		return err
	}

	defer openmfpcontext.Recover(log)
	return nil
}

func initializeTracing(ctx context.Context, log *logger.Logger) (func(ctx context.Context) error, error) {
	if defaultCfg.Tracing.Enabled {
		shutdown, err := traces.InitProvider(ctx, defaultCfg.Tracing.Collector)
		if err != nil {
			log.Fatal().Err(err).Msg("unable to start gRPC-Sidecar TracerProvider")
			return nil, err
		}
		return shutdown, nil
	}

	shutdown, err := traces.InitLocalProvider(ctx, defaultCfg.Tracing.Collector, false)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to start local TracerProvider")
		return nil, err
	}
	return shutdown, nil
}

func createServers(gatewayInstance http.Handler) (*http.Server, *http.Server, *http.Server) {
	// Main server for GraphQL
	mainMux := http.NewServeMux()
	mainMux.Handle("/", gatewayInstance)
	mainServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", appCfg.Gateway.Port),
		Handler: mainMux,
	}

	// Metrics server
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsServer := &http.Server{
		Addr:    defaultCfg.Metrics.BindAddress,
		Handler: metricsMux,
	}

	// Health server
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	healthMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	healthServer := &http.Server{
		Addr:    defaultCfg.HealthProbeBindAddress,
		Handler: healthMux,
	}

	return mainServer, metricsServer, healthServer
}

func shutdownServers(ctx context.Context, log *logger.Logger, mainServer, metricsServer, healthServer *http.Server) {
	log.Info().Msg("Shutting down HTTP servers...")

	if err := mainServer.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Main HTTP server shutdown failed")
	}

	if err := metricsServer.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Metrics HTTP server shutdown failed")
	}

	if err := healthServer.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Health HTTP server shutdown failed")
	}
}

func runServers(ctx context.Context, log *logger.Logger, gatewayInstance http.Handler) error {
	mainServer, metricsServer, healthServer := createServers(gatewayInstance)

	// Start main server (GraphQL)
	go func() {
		log.Info().Str("addr", mainServer.Addr).Msg("Starting main HTTP server")
		if err := mainServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("Error starting main HTTP server")
		}
	}()

	// Start metrics server
	go func() {
		log.Info().Str("addr", metricsServer.Addr).Msg("Starting metrics HTTP server")
		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("Error starting metrics HTTP server")
		}
	}()

	// Start health server
	go func() {
		log.Info().Str("addr", healthServer.Addr).Msg("Starting health HTTP server")
		if err := healthServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("Error starting health HTTP server")
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultCfg.ShutdownTimeout)
	defer cancel()

	shutdownServers(shutdownCtx, log, mainServer, metricsServer, healthServer)

	if closer, ok := gatewayInstance.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing gateway services")
		}
	}

	log.Info().Msg("Server shut down successfully")
	return nil
}

// setupLogger initializes the logger with the given log level
func setupLogger(logLevel string) (*logger.Logger, error) {
	loggerCfg := logger.DefaultConfig()
	loggerCfg.Name = "crdGateway"
	loggerCfg.Level = logLevel
	return logger.New(loggerCfg)
}

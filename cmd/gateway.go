package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	openmfpcontext "github.com/platform-mesh/golang-commons/context"
	"github.com/platform-mesh/golang-commons/logger"
	"github.com/platform-mesh/golang-commons/sentry"
	"github.com/platform-mesh/golang-commons/traces"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/manager"
)

var gatewayCmd = &cobra.Command{
	Use:     "gateway",
	Short:   "Run the GQL Gateway",
	Example: "go run main.go gateway",
	Run: func(_ *cobra.Command, _ []string) {
		log.Info().Str("LogLevel", log.GetLevel().String()).Msg("Starting the Gateway...")

		ctx, _, shutdown := openmfpcontext.StartContext(log, appCfg, 1*time.Second)
		defer shutdown()

		if err := initializeSentry(ctx, log); err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize Sentry")
		}

		ctrl.SetLogger(log.Logr())

		gatewayInstance, err := manager.NewGateway(ctx, log, appCfg)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create gateway")
		}

		tracingShutdown, err := initializeTracing(ctx, log)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize tracing")
		}
		defer func() {
			if err := tracingShutdown(ctx); err != nil {
				log.Error().Err(err).Msg("failed to shutdown TracerProvider")
			}
		}()

		if err := runServers(ctx, log, gatewayInstance); err != nil {
			log.Fatal().Err(err).Msg("Failed to run servers")
		}
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
		return fmt.Errorf("sentry init failed: %w", err)
	}

	defer openmfpcontext.Recover(log)
	return nil
}

func initializeTracing(ctx context.Context, log *logger.Logger) (func(ctx context.Context) error, error) {
	if defaultCfg.Tracing.Enabled {
		shutdown, err := traces.InitProvider(ctx, defaultCfg.Tracing.Collector)
		if err != nil {
			return nil, fmt.Errorf("unable to start gRPC-Sidecar TracerProvider: %w", err)
		}
		return shutdown, nil
	}

	shutdown, err := traces.InitLocalProvider(ctx, defaultCfg.Tracing.Collector, false)
	if err != nil {
		return nil, fmt.Errorf("unable to start local TracerProvider: %w", err)
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

func shutdownServers(ctx context.Context, log *logger.Logger, servers ...*http.Server) {
	log.Info().Msg("Shutting down HTTP servers...")

	for _, srv := range servers {
		if err := srv.Shutdown(ctx); err != nil {
			log.Error().Err(err).Str("addr", srv.Addr).Msg("HTTP server shutdown failed")
		}
	}
}

func runServers(ctx context.Context, log *logger.Logger, gatewayInstance http.Handler) error {
	mainServer, metricsServer, healthServer := createServers(gatewayInstance)

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		log.Info().Str("addr", mainServer.Addr).Msg("Starting main HTTP server")
		if err := mainServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("main server error: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		log.Info().Str("addr", metricsServer.Addr).Msg("Starting metrics HTTP server")
		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("metrics server error: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		log.Info().Str("addr", healthServer.Addr).Msg("Starting health HTTP server")
		if err := healthServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("health server error: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		<-egCtx.Done()
		log.Info().Msg("Shutdown signal received")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultCfg.ShutdownTimeout)
		defer cancel()

		shutdownServers(shutdownCtx, log, mainServer, metricsServer, healthServer)

		if closer, ok := gatewayInstance.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				log.Error().Err(err).Msg("Error closing gateway services")
			}
		}

		return nil
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	log.Info().Msg("Server shut down successfully")
	return nil
}

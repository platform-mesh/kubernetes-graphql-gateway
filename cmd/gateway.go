package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	openmfpcontext "github.com/openmfp/golang-commons/context"
	"github.com/openmfp/golang-commons/sentry"
	"github.com/spf13/cobra"
	restCfg "sigs.k8s.io/controller-runtime/pkg/client/config"

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

		log.Info().Str("LogLevel", log.GetLevel().String()).Msg("Starting server...")

		ctx, _, shutdown := openmfpcontext.StartContext(log, appCfg, 1*time.Second)
		defer shutdown()

		if defaultCfg.Sentry.Dsn != "" {
			err := sentry.Start(ctx,
				defaultCfg.Sentry.Dsn, defaultCfg.Environment, defaultCfg.Region,
				defaultCfg.Image.Name, defaultCfg.Image.Tag,
			)
			if err != nil {
				log.Fatal().Err(err).Msg("Sentry init failed")
			}

			defer openmfpcontext.Recover(log)
		}

		// Get Kubernetes restCfg
		restCfg, err := restCfg.GetConfig()
		if err != nil {
			log.Fatal().Err(err).Msg("Error getting Kubernetes restCfg, exiting")
		}

		// Initialize Manager
		managerInstance, err := manager.NewManager(log, restCfg, appCfg)
		if err != nil {
			log.Error().Err(err).Msg("Error creating manager")
			return fmt.Errorf("failed to create manager: %w", err)
		}

		// Set up HTTP handler
		http.Handle("/", managerInstance)

		// Start HTTP server with context
		server := &http.Server{
			Addr:    fmt.Sprintf(":%s", appCfg.Gateway.Port),
			Handler: nil,
		}

		// Start the HTTP server in a goroutine so that we can listen for shutdown signals
		go func() {
			err := server.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error().Err(err).Msg("Error starting HTTP server")
			}
		}()

		// Wait for shutdown signal via the context
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultCfg.ShutdownTimeout) // ctx is closed, we need a new one
		defer cancel()
		log.Info().Msg("Shutting down HTTP server...")
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatal().Err(err).Msg("HTTP server shutdown failed")
		}

		// Call the shutdown cleanup
		shutdown()

		log.Info().Msg("Server shut down successfully")
		return nil
	},
}

// setupLogger initializes the logger with the given log level
func setupLogger(logLevel string) (*logger.Logger, error) {
	loggerCfg := logger.DefaultConfig()
	loggerCfg.Name = "crdGateway"
	loggerCfg.Level = logLevel
	return logger.New(loggerCfg)
}

package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	utilscontext "github.com/platform-mesh/kubernetes-graphql-gateway/gateway/utils/context"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ServerConfig struct {
	// Gateway is the main server interface to handle GraphQL requests. Its http server compliant component
	Gateway http.Handler

	CORSConfig CORSConfig

	// Addr is the address the server listens on
	Addr string
}

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedHeaders   []string
	AllowCredentials bool
}

type Server struct {
	Server *http.Server
}

// NewServer creates a new HTTP server with the provided configuration
// It main server, used to serve the GraphQL API, health checks, and metrics
func NewServer(c ServerConfig) (*Server, error) {
	s := http.NewServeMux()

	s.Handle("/api/clusters/{clusterName}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clusterName := r.PathValue("clusterName")

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized: missing Authorization header", http.StatusUnauthorized)
			return
		}
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Unauthorized: invalid Authorization header format", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			http.Error(w, "Unauthorized: empty bearer token", http.StatusUnauthorized)
			return
		}

		ctx := utilscontext.SetToken(r.Context(), token)
		ctx = utilscontext.SetCluster(ctx, clusterName)
		c.Gateway.ServeHTTP(w, r.WithContext(ctx))
	}))

	// TODO: Add middleware for logging, metrics, tracing, etc.

	// Health and metrics endpoints
	s.Handle("/healthz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	s.Handle("/readyz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	s.Handle("/metrics", promhttp.Handler())

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   c.CORSConfig.AllowedOrigins,
		AllowedHeaders:   c.CORSConfig.AllowedHeaders,
		AllowCredentials: c.CORSConfig.AllowCredentials,
	})

	return &Server{
		Server: &http.Server{
			Handler: corsHandler.Handler(s),
			Addr:    c.Addr,
		},
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	logger := log.FromContext(ctx)

	logger.WithValues("addr", s.Server.Addr).Info("Starting HTTP server")

	// Gracefully shut down the HTTP server when the context is cancelled
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.Server.Shutdown(shutdownCtx); err != nil {
			logger.Error(err, "HTTP server shutdown error")
		}
	}()

	if err := s.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

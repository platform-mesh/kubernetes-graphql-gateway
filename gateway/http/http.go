package http

import (
	"context"
	"net/http"

	utilscontext "github.com/platform-mesh/kubernetes-graphql-gateway/gateway/utils/context"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Server interface {
	Run(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type ServerConfig struct {
	// Gateway is the main server interface to handle GraphQL requests. Its http server compliant component
	Gateway http.Handler

	// Addr is the address the server listens on
	Addr string
}

type server struct {
	Server *http.Server
}

// NewServer creates a new HTTP server with the provided configuration
// It main server, used to serve the GraphQL API, health checks, and metrics
func NewServer(c ServerConfig) (Server, error) {
	s := http.NewServeMux()

	// Route with cluster name path parameter using Go 1.22+ pattern matching
	// Pattern: /api/clusters/{clusterName} where {clusterName} captures the path segment
	s.Handle("/api/clusters/{clusterName}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clusterName := r.PathValue("clusterName")
		// Add cluster name to request context for downstream handlers
		ctx := utilscontext.SetCluster(r.Context(), clusterName)
		c.Gateway.ServeHTTP(w, r.WithContext(ctx))
	}))

	// TODO: Add AccessCluster separate endpoint. Something like
	// /api/remote-clusters/{clusterName}/graphql (need better naming...)
	// This would allow for better separation of concerns and clearer routing

	// TODO: Add middleware for logging, metrics, tracing, etc.

	// Health and metrics endpoints
	s.Handle("/healthz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	s.Handle("/readyz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	s.Handle("/metrics", promhttp.Handler())

	return &server{
		Server: &http.Server{
			Handler: s,
			Addr:    c.Addr,
		},
	}, nil
}

func (s *server) Run(ctx context.Context) error {
	logger := log.FromContext(ctx)

	logger.WithValues("addr", s.Server.Addr).Info("Starting HTTP server")
	return s.Server.ListenAndServe()
}

func (s *server) Shutdown(ctx context.Context) error {
	return s.Server.Shutdown(ctx)
}

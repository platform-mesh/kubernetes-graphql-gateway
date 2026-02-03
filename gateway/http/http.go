package http

import (
	"context"
	"net/http"
	"strings"

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

	// Pattern: /api/clusters/{clusterName} where {clusterName} captures the path segment
	s.Handle("/api/clusters/{clusterName}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clusterName := r.PathValue("clusterName")
		// FIXME: for now lets implement the token extraction here until a better place is found

		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")

		ctx := utilscontext.SetToken(r.Context(), token)

		// Add cluster name to request context for downstream handlers
		ctx = utilscontext.SetCluster(ctx, clusterName)
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
	return s.Server.ListenAndServe()
}

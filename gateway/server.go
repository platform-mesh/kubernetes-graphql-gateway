package gateway

import (
	"context"
	"sync"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/http"
	"k8s.io/klog/v2"
)

type Server struct {
	HTTPServer http.Server
	Gateway    *gateway.Service
}

func NewServer(c *Config) (Server, error) {
	return Server{
		HTTPServer: c.HTTPServer,
		Gateway:    c.Gateway,
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	logger.Info("Starting Gateway Server")

	wg := sync.WaitGroup{}
	go func() {
		defer wg.Done()
		if err := s.Gateway.Run(ctx); err != nil {
			logger.Error(err, "Gateway encountered an error")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.HTTPServer.Run(ctx); err != nil {
			logger.Error(err, "HTTP server encountered an error")
		}
	}()

	wg.Wait()
	return nil
}

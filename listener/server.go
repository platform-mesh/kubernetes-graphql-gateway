package listener

import (
	"context"
	"fmt"

	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/controllers/namespaces"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"
)

type Server struct {
	Config *Config

	Controllers
}

type Controllers struct {
	// Namespaces reconciler is used when we are operating in kubernetes mode
	Namespaces *namespaces.NamespaceReconciler
}

func NewServer(ctx context.Context, c *Config) (*Server, error) {
	logger := klog.FromContext(ctx)
	logger.Info("Setting up Listener Server controllers")

	s := &Server{
		Config: c,
	}

	opts := controller.TypedOptions[mcreconcile.Request]{}
	var err error
	s.Namespaces, err = namespaces.NewNamespaceReconciler(
		ctx,
		s.Config.Manager,
		opts,
		s.Config.IOHandler,
		s.Config.SchemaResolver,
		c.Options.AnchorNamespace,
		c.Options.ClusterMetadataFunc,
		c.Options.ClusterURLResolverFunc,
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up Namespace Controller: %w", err)
	}
	if err := s.Namespaces.SetupWithManager(s.Config.Manager); err != nil {
		return nil, fmt.Errorf("error setting up Namespace controller with manager: %w", err)
	}

	return s, nil
}

func (s *Server) Run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	logger.Info("Starting Listener")

	return s.Config.Manager.Start(ctx)
}

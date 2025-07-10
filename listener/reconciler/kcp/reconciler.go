package kcp

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	kcpctrl "sigs.k8s.io/controller-runtime/pkg/kcp"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/workspacefile"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler"
)

type KCPReconciler struct {
	mgr ctrl.Manager
	log *logger.Logger
}

func NewKCPReconciler(
	appCfg config.Config,
	opts reconciler.ReconcilerOpts,
	log *logger.Logger,
) (*KCPReconciler, error) {
	log.Info().Msg("Setting up KCP reconciler with workspace discovery")

	// Create KCP-aware manager
	mgr, err := kcpctrl.NewClusterAwareManager(opts.Config, opts.ManagerOpts)
	if err != nil {
		log.Error().Err(err).Msg("failed to create KCP-aware manager")
		return nil, err
	}

	// Create IO handler for schema files
	ioHandler, err := workspacefile.NewIOHandler(appCfg.OpenApiDefinitionsPath)
	if err != nil {
		log.Error().Err(err).Msg("failed to create IO handler")
		return nil, err
	}

	// Create schema resolver
	schemaResolver := apischema.NewResolver()

	// Create cluster path resolver
	clusterPathResolver, err := NewClusterPathResolver(opts.Config, opts.Scheme)
	if err != nil {
		log.Error().Err(err).Msg("failed to create cluster path resolver")
		return nil, err
	}

	// Create discovery factory
	discoveryFactory, err := NewDiscoveryFactory(opts.Config)
	if err != nil {
		log.Error().Err(err).Msg("failed to create discovery factory")
		return nil, err
	}

	// Setup APIBinding reconciler
	apiBindingReconciler := &APIBindingReconciler{
		Client:              mgr.GetClient(),
		Scheme:              opts.Scheme,
		RestConfig:          opts.Config,
		IOHandler:           ioHandler,
		DiscoveryFactory:    discoveryFactory,
		APISchemaResolver:   schemaResolver,
		ClusterPathResolver: clusterPathResolver,
		Log:                 log,
	}

	// Setup the controller with cluster context - this is crucial for req.ClusterName
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&kcpapis.APIBinding{}).
		Complete(kcpctrl.WithClusterInContext(apiBindingReconciler)); err != nil {
		log.Error().Err(err).Msg("failed to setup APIBinding controller")
		return nil, err
	}

	log.Info().Msg("Successfully configured KCP reconciler with workspace discovery")

	return &KCPReconciler{
		mgr: mgr,
		log: log,
	}, nil
}

func (r *KCPReconciler) GetManager() ctrl.Manager {
	return r.mgr
}

func (r *KCPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// This method is not used - reconciliation is handled by the APIBinding controller
	return ctrl.Result{}, nil
}

func (r *KCPReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Controllers are already set up in the constructor
	return nil
}

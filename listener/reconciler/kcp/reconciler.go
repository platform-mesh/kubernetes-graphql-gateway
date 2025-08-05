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
	mgr                        ctrl.Manager
	apiBindingReconciler       *APIBindingReconciler
	virtualWorkspaceReconciler *VirtualWorkspaceReconciler
	configWatcher              *ConfigWatcher
	log                        *logger.Logger
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
	schemaResolver := apischema.NewResolver(log)

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

	// Create APIBinding reconciler (but don't set up controller yet)
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

	// Setup virtual workspace components
	virtualWSManager := NewVirtualWorkspaceManager(appCfg)
	virtualWorkspaceReconciler := NewVirtualWorkspaceReconciler(
		virtualWSManager,
		ioHandler,
		schemaResolver,
		log,
	)

	configWatcher, err := NewConfigWatcher(virtualWSManager, log)
	if err != nil {
		log.Error().Err(err).Msg("failed to create config watcher")
		return nil, err
	}

	reconcilerInstance := &KCPReconciler{
		mgr:                        mgr,
		apiBindingReconciler:       apiBindingReconciler,
		virtualWorkspaceReconciler: virtualWorkspaceReconciler,
		configWatcher:              configWatcher,
		log:                        log,
	}

	log.Info().Msg("Successfully configured KCP reconciler with workspace discovery")
	return reconcilerInstance, nil
}

func (r *KCPReconciler) GetManager() ctrl.Manager {
	return r.mgr
}

func (r *KCPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// This method is required by the reconciler.CustomReconciler interface but is not used directly.
	// Actual reconciliation is handled by the APIBinding controller set up in SetupWithManager().
	// KCPReconciler acts as a coordinator/manager rather than a direct reconciler.
	return ctrl.Result{}, nil
}

func (r *KCPReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Handle cases where the reconciler wasn't properly initialized (e.g., in tests)
	if r.apiBindingReconciler == nil {
		return nil
	}

	// Setup the APIBinding controller with cluster context - this is crucial for req.ClusterName
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&kcpapis.APIBinding{}).
		Complete(kcpctrl.WithClusterInContext(r.apiBindingReconciler)); err != nil {
		r.log.Error().Err(err).Msg("failed to setup APIBinding controller")
		return err
	}

	r.log.Info().Msg("Successfully set up APIBinding controller")
	return nil
}

// StartVirtualWorkspaceWatching starts watching virtual workspace configuration
func (r *KCPReconciler) StartVirtualWorkspaceWatching(ctx context.Context, configPath string) error {
	if configPath == "" {
		r.log.Info().Msg("no virtual workspace config path provided, skipping virtual workspace watching")
		return nil
	}

	r.log.Info().Str("configPath", configPath).Msg("starting virtual workspace configuration watching")

	// Start config watcher with a wrapper function
	changeHandler := func(config *VirtualWorkspacesConfig) {
		if err := r.virtualWorkspaceReconciler.ReconcileConfig(ctx, config); err != nil {
			r.log.Error().Err(err).Msg("failed to reconcile virtual workspaces config")
		}
	}
	return r.configWatcher.Watch(ctx, configPath, changeHandler)
}

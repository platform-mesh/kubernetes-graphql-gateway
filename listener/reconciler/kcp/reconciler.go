package kcp

import (
	"context"
	"fmt"

	"github.com/platform-mesh/golang-commons/logger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common/config"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"

	kcpapis "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	kcptenancy "github.com/kcp-dev/sdk/apis/tenancy/v1alpha1"

	"github.com/kcp-dev/multicluster-provider/initializingworkspaces"
)

type KCPReconciler struct {
	mgr                        mcmanager.Manager
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

	if opts.Scheme == nil {
		return nil, fmt.Errorf("scheme should not be nil")
	}

	ioHandler, err := workspacefile.NewIOHandler(appCfg.OpenApiDefinitionsPath)
	if err != nil {
		log.Error().Err(err).Msg("failed to create IO handler")
		return nil, err
	}

	utilruntime.Must(kcptenancy.AddToScheme(opts.Scheme))

	// Create multi-cluster manager
	provider, err := initializingworkspaces.New(opts.Config, "root:orgs", initializingworkspaces.Options{Scheme: opts.Scheme})
	if err != nil {
		log.Error().Err(err).Msg("unable to construct cluster provider")
		return nil, err
	}

	mgr, err := mcmanager.New(opts.Config, provider, opts.ManagerOpts)
	if err != nil {
		log.Error().Err(err).Msg("failed to create multi-cluster manager")
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
		Client:              mgr.GetLocalManager().GetClient(),
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

func (r *KCPReconciler) GetManager() mcmanager.Manager {
	return r.mgr
}

func (r *KCPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// This method is required by the reconciler.CustomReconciler interface but is not used directly.
	// Actual reconciliation is handled by the APIBinding controller set up in SetupWithManager().
	// KCPReconciler acts as a coordinator/manager rather than a direct reconciler.
	return ctrl.Result{}, nil
}

func (r *KCPReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	// Handle cases where the reconciler wasn't properly initialized (e.g., in tests)
	if r.apiBindingReconciler == nil {
		return nil
	}

	// Setup the APIBinding controller
	if err := ctrl.NewControllerManagedBy(mgr.GetLocalManager()).
		For(&kcpapis.APIBinding{}).
		Complete(r.apiBindingReconciler); err != nil {
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
	changeHandler := func(config *VirtualWorkspacesConfig) error {
		if err := r.virtualWorkspaceReconciler.ReconcileConfig(ctx, config); err != nil {
			r.log.Error().Err(err).Msg("failed to reconcile virtual workspaces config")
			return err
		}
		return nil
	}
	return r.configWatcher.Watch(ctx, configPath, changeHandler)
}

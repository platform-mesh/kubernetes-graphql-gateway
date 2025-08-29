package kcp

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	"github.com/kcp-dev/multicluster-provider/apiexport"
	"github.com/platform-mesh/golang-commons/logger"
	appconfig "github.com/platform-mesh/kubernetes-graphql-gateway/common/config"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler"

	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"
)

type KCPReconciler struct {
	mcMgr                      mcmanager.Manager
	provider                   *apiexport.Provider
	ioHandler                  workspacefile.IOHandler
	schemaResolver             apischema.Resolver
	virtualWorkspaceReconciler *VirtualWorkspaceReconciler
	configWatcher              *ConfigWatcher
	log                        *logger.Logger
	manager                    ctrl.Manager // Pre-created manager wrapper
}

func NewKCPReconciler(
	appCfg appconfig.Config,
	opts reconciler.ReconcilerOpts,
	log *logger.Logger,
) (*KCPReconciler, error) {
	// Validate inputs first before using the logger
	if log == nil {
		return nil, fmt.Errorf("logger should not be nil")
	}
	if opts.Scheme == nil {
		return nil, fmt.Errorf("scheme should not be nil")
	}

	log.Info().Msg("Setting up KCP reconciler with multicluster-provider")

	// Create IO handler for schema files
	ioHandler, err := workspacefile.NewIOHandler(appCfg.OpenApiDefinitionsPath)
	if err != nil {
		log.Error().Err(err).Msg("failed to create IO handler")
		return nil, err
	}

	// Create schema resolver
	schemaResolver := apischema.NewResolver(log)

	// Create the apiexport provider for multicluster-runtime
	provider, err := apiexport.New(opts.Config, apiexport.Options{
		Scheme: opts.Scheme,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to create apiexport provider")
		return nil, err
	}

	// Create multicluster manager
	mcMgr, err := mcmanager.New(opts.Config, provider, opts.ManagerOpts)
	if err != nil {
		log.Error().Err(err).Msg("failed to create multicluster manager")
		return nil, err
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
		mcMgr:                      mcMgr,
		provider:                   provider,
		ioHandler:                  ioHandler,
		schemaResolver:             schemaResolver,
		virtualWorkspaceReconciler: virtualWorkspaceReconciler,
		configWatcher:              configWatcher,
		log:                        log,
	}

	// Create the manager wrapper that handles KCP-specific startup
	reconcilerInstance.manager = &kcpManagerWrapper{
		Manager:    mcMgr.GetLocalManager(),
		reconciler: reconcilerInstance,
	}

	log.Info().Msg("Successfully configured KCP reconciler with multicluster-provider")
	return reconcilerInstance, nil
}

func (r *KCPReconciler) GetManager() ctrl.Manager {
	return r.manager
}

func (r *KCPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// This method is required by the reconciler.CustomReconciler interface but is not used directly.
	// Actual reconciliation is handled by the multicluster controller set up in SetupWithManager().
	// KCPReconciler acts as a coordinator/manager rather than a direct reconciler.
	return ctrl.Result{}, nil
}

func (r *KCPReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := mcbuilder.ControllerManagedBy(r.mcMgr).
		Named("kcp-apibinding-schema-controller").
		For(&kcpapis.APIBinding{}).
		Complete(mcreconcile.Func(r.reconcileAPIBinding))
	if err != nil {
		r.log.Error().Err(err).Msg("failed to setup multicluster APIBinding controller")
		return err
	}

	r.log.Info().Msg("Successfully set up multicluster APIBinding controller")
	return nil
}

// reconcileAPIBinding handles APIBinding reconciliation across multiple clusters
func (r *KCPReconciler) reconcileAPIBinding(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	logger := r.log.With().Str("cluster", req.ClusterName).Str("name", req.Name).Logger()

	// Get the cluster from the multicluster manager
	cluster, err := r.mcMgr.GetCluster(ctx, req.ClusterName)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get cluster from multicluster manager")
		return ctrl.Result{}, fmt.Errorf("failed to get cluster: %w", err)
	}

	client := cluster.GetClient()

	// Check if APIBinding still exists (it might have been deleted)
	apiBinding := &kcpapis.APIBinding{}
	err = client.Get(ctx, req.NamespacedName, apiBinding)
	if err != nil {
		logger.Info().Msg("APIBinding not found, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Generate and write schema for this cluster
	clusterPath := req.ClusterName
	err = r.generateAndWriteSchema(ctx, clusterPath, cluster)
	if err != nil {
		logger.Error().Err(err).Msg("failed to generate and write schema")
		return ctrl.Result{}, err
	}

	logger.Info().Msg("successfully reconciled APIBinding schema")
	return ctrl.Result{}, nil
}

// generateAndWriteSchema generates the OpenAPI schema for a cluster and writes it to disk
func (r *KCPReconciler) generateAndWriteSchema(ctx context.Context, clusterPath string, clusterObj cluster.Cluster) error {
	// Create discovery client and REST mapper from the cluster's config
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(clusterObj.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}

	// Generate current schema
	currentSchema, err := generateSchemaWithMetadata(
		SchemaGenerationParams{
			ClusterPath:     clusterPath,
			DiscoveryClient: discoveryClient,
			RESTMapper:      clusterObj.GetRESTMapper(),
		},
		r.schemaResolver,
		r.log,
	)
	if err != nil {
		return fmt.Errorf("failed to generate schema: %w", err)
	}

	// Read existing schema (if it exists)
	savedSchema, err := r.ioHandler.Read(clusterPath)
	if err != nil && !strings.Contains(err.Error(), "file does not exist") && !strings.Contains(err.Error(), "no such file") {
		return fmt.Errorf("failed to read existing schema: %w", err)
	}

	// Write if file doesn't exist or content has changed
	if err != nil || !bytes.Equal(currentSchema, savedSchema) {
		err = r.ioHandler.Write(currentSchema, clusterPath)
		if err != nil {
			return fmt.Errorf("failed to write schema: %w", err)
		}
		r.log.Info().Str("clusterPath", clusterPath).Msg("schema file updated")
	}

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

// Start starts the provider and multicluster manager
func (r *KCPReconciler) Start(ctx context.Context) error {
	r.log.Info().Msg("Starting KCP reconciler with multicluster-provider")

	// Start the provider
	go func() {
		err := r.provider.Run(ctx, r.mcMgr)
		if err != nil {
			r.log.Error().Err(err).Msg("provider failed to run")
		}
	}()

	// Start the multicluster manager
	return r.mcMgr.Start(ctx)
}

// kcpManagerWrapper wraps the local manager and ensures proper startup of KCP components
type kcpManagerWrapper struct {
	ctrl.Manager
	reconciler *KCPReconciler
}

// Start starts both the provider and the multicluster manager
func (w *kcpManagerWrapper) Start(ctx context.Context) error {
	return w.reconciler.Start(ctx)
}

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
	commoncluster "github.com/platform-mesh/kubernetes-graphql-gateway/common/cluster"
	appconfig "github.com/platform-mesh/kubernetes-graphql-gateway/common/config"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler"

	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"
)

// KCPManager manages multicluster KCP components and schema generation.
// It coordinates the multicluster provider, virtual workspaces, and API binding controllers.
// Unlike traditional reconcilers, KCPManager acts as a coordinator that sets up multicluster
// controllers rather than directly reconciling resources.
type KCPManager struct {
	mcMgr                      mcmanager.Manager
	provider                   *apiexport.Provider
	ioHandler                  workspacefile.IOHandler
	schemaResolver             apischema.Resolver
	virtualWorkspaceReconciler *VirtualWorkspaceReconciler
	configWatcher              *ConfigWatcher
	log                        *logger.Logger
	manager                    ctrl.Manager          // Local controller-runtime manager
	clusterManager             commoncluster.Manager // Unified cluster manager for gateway integration
}

func NewKCPManager(
	appCfg appconfig.Config,
	opts reconciler.ReconcilerOpts,
	log *logger.Logger,
) (*KCPManager, error) {
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

	managerInstance := &KCPManager{
		mcMgr:                      mcMgr,
		provider:                   provider,
		ioHandler:                  ioHandler,
		schemaResolver:             schemaResolver,
		virtualWorkspaceReconciler: virtualWorkspaceReconciler,
		configWatcher:              configWatcher,
		log:                        log,
		manager:                    mcMgr.GetLocalManager(),                     // Use the local manager directly
		clusterManager:             commoncluster.NewMulticlusterManager(mcMgr), // Expose multicluster manager for gateway
	}

	log.Info().Msg("Successfully configured KCP manager with multicluster-provider")
	return managerInstance, nil
}

func (m *KCPManager) GetManager() ctrl.Manager {
	return m.manager
}

// GetClusterManager returns the unified cluster manager for gateway integration
func (m *KCPManager) GetClusterManager() commoncluster.Manager {
	return m.clusterManager
}

func (m *KCPManager) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// This method is required by the reconciler.ControllerProvider interface but is not used directly.
	// Actual reconciliation is handled by the multicluster controller set up in SetupWithManager().
	// KCPManager acts as a coordinator/manager rather than a direct reconciler.
	return ctrl.Result{}, nil
}

func (m *KCPManager) SetupWithManager(mgr ctrl.Manager) error {
	// Setup the multicluster APIBinding controller
	err := mcbuilder.ControllerManagedBy(m.mcMgr).
		Named("kcp-apibinding-schema-controller").
		For(&kcpapis.APIBinding{}).
		Complete(mcreconcile.Func(m.reconcileAPIBinding))
	if err != nil {
		m.log.Error().Err(err).Msg("failed to setup multicluster APIBinding controller")
		return err
	}

	// Add the provider as a runnable to the manager so it starts with the manager
	err = mgr.Add(&providerRunnable{
		provider: m.provider,
		mcMgr:    m.mcMgr,
		log:      m.log,
	})
	if err != nil {
		m.log.Error().Err(err).Msg("failed to add provider runnable to manager")
		return err
	}

	m.log.Info().Msg("Successfully set up multicluster APIBinding controller and provider")
	return nil
}

// reconcileAPIBinding handles APIBinding reconciliation across multiple clusters
func (m *KCPManager) reconcileAPIBinding(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	logger := m.log.With().Str("cluster", req.ClusterName).Str("name", req.Name).Logger()

	// Get the cluster from the multicluster manager
	cluster, err := m.mcMgr.GetCluster(ctx, req.ClusterName)
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
	err = m.generateAndWriteSchema(ctx, clusterPath, cluster)
	if err != nil {
		logger.Error().Err(err).Msg("failed to generate and write schema")
		return ctrl.Result{}, err
	}

	logger.Info().Msg("successfully reconciled APIBinding schema")
	return ctrl.Result{}, nil
}

// generateAndWriteSchema generates the OpenAPI schema for a cluster and writes it to disk
func (m *KCPManager) generateAndWriteSchema(ctx context.Context, clusterPath string, clusterObj cluster.Cluster) error {
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
		m.schemaResolver,
		m.log,
	)
	if err != nil {
		return fmt.Errorf("failed to generate schema: %w", err)
	}

	// Read existing schema (if it exists)
	savedSchema, err := m.ioHandler.Read(clusterPath)
	if err != nil && !strings.Contains(err.Error(), "file does not exist") && !strings.Contains(err.Error(), "no such file") {
		return fmt.Errorf("failed to read existing schema: %w", err)
	}

	// Write if file doesn't exist or content has changed
	if err != nil || !bytes.Equal(currentSchema, savedSchema) {
		err = m.ioHandler.Write(currentSchema, clusterPath)
		if err != nil {
			return fmt.Errorf("failed to write schema: %w", err)
		}
		m.log.Info().Str("clusterPath", clusterPath).Msg("schema file updated")
	}

	return nil
}

// StartVirtualWorkspaceWatching starts watching virtual workspace configuration
func (m *KCPManager) StartVirtualWorkspaceWatching(ctx context.Context, configPath string) error {
	if configPath == "" {
		m.log.Info().Msg("no virtual workspace config path provided, skipping virtual workspace watching")
		return nil
	}

	m.log.Info().Str("configPath", configPath).Msg("starting virtual workspace configuration watching")

	// Start config watcher with a wrapper function
	changeHandler := func(config *VirtualWorkspacesConfig) {
		if err := m.virtualWorkspaceReconciler.ReconcileConfig(ctx, config); err != nil {
			m.log.Error().Err(err).Msg("failed to reconcile virtual workspaces config")
		}
	}
	return m.configWatcher.Watch(ctx, configPath, changeHandler)
}

// providerRunnable wraps the apiexport provider to make it compatible with controller-runtime manager
type providerRunnable struct {
	provider *apiexport.Provider
	mcMgr    mcmanager.Manager
	log      *logger.Logger
}

func (p *providerRunnable) Start(ctx context.Context) error {
	p.log.Info().Msg("Starting KCP provider with multicluster manager")
	return p.provider.Run(ctx, p.mcMgr)
}

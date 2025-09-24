package kcp

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	manager                    ctrl.Manager // Local controller-runtime manager
	clusterPathResolver        *ClusterPathResolverProvider
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
	// Configure the provider to use the APIExport endpoint
	// The multicluster-provider needs to connect to the specific APIExport endpoint
	// to discover workspaces, not the base KCP host
	apiexportConfig := rest.CopyConfig(opts.Config)

	// Extract base KCP host from kubeconfig, stripping any APIExport paths
	// This ensures we work with both base KCP hosts and APIExport URLs in kubeconfig
	originalHost := opts.Config.Host
	baseHost := stripAPIExportPath(originalHost)

	log.Info().
		Str("originalHost", originalHost).
		Str("baseHost", baseHost).
		Msg("Extracted base KCP host from kubeconfig")

	// Construct the APIExport URL for multicluster-provider discovery
	// We need to extract the cluster hash from the original APIExport URL if present
	clusterHash := extractClusterHashFromAPIExportURL(originalHost)
	if clusterHash == "" {
		// Fallback to a known cluster hash - this should be made configurable
		clusterHash = "1mx3340lwq4c8kkw"
		log.Warn().Str("fallbackHash", clusterHash).Msg("Could not extract cluster hash from kubeconfig, using fallback")
	}

	apiexportURL := fmt.Sprintf("%s/services/apiexport/%s/core.platform-mesh.io/", baseHost, clusterHash)

	log.Info().Str("baseHost", baseHost).Str("apiexportURL", apiexportURL).Msg("Using APIExport URL for multicluster provider")
	apiexportConfig.Host = apiexportURL

	provider, err := apiexport.New(apiexportConfig, apiexport.Options{
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

	// Create cluster path resolver for workspace path resolution
	clusterPathResolver, err := NewClusterPathResolver(mcMgr.GetLocalManager().GetConfig(), mcMgr.GetLocalManager().GetScheme(), log)
	if err != nil {
		log.Error().Err(err).Msg("failed to create cluster path resolver")
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
		manager:                    mcMgr.GetLocalManager(), // Use the local manager directly
		clusterPathResolver:        clusterPathResolver,
	}

	log.Info().Msg("Successfully configured KCP manager with multicluster-provider")
	return managerInstance, nil
}

func (m *KCPManager) GetManager() ctrl.Manager {
	return m.manager
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

	// Resolve cluster name (hash) to workspace path (e.g., orgs:openmfp:default)
	// This ensures compatibility with GraphQL gateway which expects workspace names
	workspacePath, err := m.resolveWorkspacePath(ctx, req.ClusterName, client)
	if err != nil {
		logger.Error().Err(err).Str("clusterName", req.ClusterName).Msg("failed to resolve cluster name to workspace path")
		return ctrl.Result{}, fmt.Errorf("failed to resolve workspace path: %w", err)
	}

	// If we got the same value back (cluster name), try alternative approach using cluster config
	if workspacePath == req.ClusterName {
		logger.Debug().Str("clusterName", req.ClusterName).Str("configHost", cluster.GetConfig().Host).Msg("LogicalCluster approach returned cluster name, trying config-based approach")
		configBasedPath, configErr := PathForClusterFromConfig(req.ClusterName, cluster.GetConfig())
		if configErr == nil && configBasedPath != req.ClusterName {
			workspacePath = configBasedPath
			logger.Info().Str("clusterName", req.ClusterName).Str("workspacePath", workspacePath).Str("configHost", cluster.GetConfig().Host).Msg("resolved workspace path from cluster config")
		} else {
			// Log the cluster config URL for debugging
			logger.Info().Str("clusterName", req.ClusterName).Str("configHost", cluster.GetConfig().Host).Str("workspacePath", workspacePath).Msg("using cluster name as workspace path (no LogicalCluster or config-based resolution available)")
		}
	} else {
		logger.Info().Str("clusterName", req.ClusterName).Str("workspacePath", workspacePath).Msg("resolved cluster name to workspace path from LogicalCluster")
	}

	// Generate and write schema for this cluster using the workspace path
	// Create a direct workspace client instead of using the APIExport cluster
	// This ensures we get the full API resource list from the workspace, not just exported APIs
	err = m.generateAndWriteSchemaForWorkspace(ctx, workspacePath, req.ClusterName)
	if err != nil {
		logger.Error().Err(err).Msg("failed to generate and write schema")
		return ctrl.Result{}, err
	}

	logger.Info().Str("workspacePath", workspacePath).Msg("successfully reconciled APIBinding schema")
	return ctrl.Result{}, nil
}

// generateAndWriteSchemaForWorkspace generates the OpenAPI schema for a workspace using direct access
func (m *KCPManager) generateAndWriteSchemaForWorkspace(ctx context.Context, workspacePath, clusterName string) error {
	// Create direct workspace config for discovery
	// This ensures we get the full API resource list from the workspace, not just exported APIs
	workspaceConfig, err := ConfigForKCPCluster(clusterName, m.mcMgr.GetLocalManager().GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create workspace config: %w", err)
	}

	// WORKAROUND: Use the original approach from main branch
	// Create discovery client but ensure it doesn't make /api requests to KCP front proxy
	// Use the existing discovery factory which should handle KCP properly
	discoveryFactory, err := NewDiscoveryFactory(workspaceConfig)
	if err != nil {
		return fmt.Errorf("failed to create discovery factory: %w", err)
	}

	discoveryClient, err := discoveryFactory.ClientForCluster(clusterName)
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}

	restMapper, err := discoveryFactory.RestMapperForCluster(clusterName)
	if err != nil {
		return fmt.Errorf("failed to create REST mapper: %w", err)
	}

	// Use direct workspace URLs like the main branch for gateway compatibility
	// The multicluster-provider is only used for workspace discovery in the listener
	// The gateway will use standard Kubernetes clients with direct workspace URLs
	baseConfig := m.mcMgr.GetLocalManager().GetConfig()

	// Strip APIExport path from the base config host to get the clean KCP host
	baseHost := stripAPIExportPath(baseConfig.Host)

	// Construct direct workspace URL like main branch: /clusters/{workspace}
	directWorkspaceHost := fmt.Sprintf("%s/clusters/%s", baseHost, workspacePath)

	m.log.Info().
		Str("clusterName", clusterName).
		Str("workspacePath", workspacePath).
		Str("baseHost", baseHost).
		Str("directWorkspaceHost", directWorkspaceHost).
		Msg("Using direct workspace URL for gateway compatibility (same as main branch)")

	// Generate current schema using direct workspace access
	currentSchema, err := generateSchemaWithMetadata(
		SchemaGenerationParams{
			ClusterPath:     workspacePath,
			DiscoveryClient: discoveryClient,
			RESTMapper:      restMapper,
			HostOverride:    directWorkspaceHost, // Use direct workspace URL like main branch
		},
		m.schemaResolver,
		m.log,
	)
	if err != nil {
		return fmt.Errorf("failed to generate schema: %w", err)
	}

	// Read existing schema (if it exists)
	savedSchema, err := m.ioHandler.Read(workspacePath)
	if err != nil && !strings.Contains(err.Error(), "file does not exist") && !strings.Contains(err.Error(), "no such file") {
		return fmt.Errorf("failed to read existing schema: %w", err)
	}

	// Write if file doesn't exist or content has changed
	if err != nil || !bytes.Equal(currentSchema, savedSchema) {
		err = m.ioHandler.Write(currentSchema, workspacePath)
		if err != nil {
			return fmt.Errorf("failed to write schema: %w", err)
		}
		m.log.Info().Str("clusterPath", workspacePath).Msg("schema file updated")
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

// resolveWorkspacePath resolves a cluster name/hash to a human-readable workspace path
func (m *KCPManager) resolveWorkspacePath(ctx context.Context, clusterName string, clusterClient client.Client) (string, error) {
	// For multicluster-provider, we need to create a client that connects directly to the cluster hash
	// The clusterClient passed in might not be correctly configured for the specific cluster

	// Get a client specifically for this cluster using the pre-initialized resolver
	specificClusterClient, err := m.clusterPathResolver.ClientForCluster(clusterName)
	if err != nil {
		// Use the resolver with the provided client as fallback
		return m.clusterPathResolver.PathForCluster(clusterName, clusterClient)
	}

	// Use the cluster-specific client to resolve the workspace path with logger
	workspacePath, err := m.clusterPathResolver.PathForCluster(clusterName, specificClusterClient)
	if err != nil {
		return clusterName, err
	}

	return workspacePath, nil
}

// providerRunnable wraps the apiexport provider to make it compatible with controller-runtime manager
type providerRunnable struct {
	provider *apiexport.Provider
	mcMgr    mcmanager.Manager
	log      *logger.Logger
}

func (p *providerRunnable) Start(ctx context.Context) error {
	p.log.Info().Msg("Starting KCP provider with multicluster manager")

	// Add a small delay to allow KCP services to be ready
	p.log.Info().Msg("Waiting for KCP services to be ready...")
	select {
	case <-time.After(5 * time.Second):
		// Continue after delay
	case <-ctx.Done():
		return ctx.Err()
	}

	// Run the provider with error handling to prevent listener crash
	if err := p.provider.Run(ctx, p.mcMgr); err != nil {
		p.log.Error().Err(err).Msg("KCP provider encountered an error, but continuing to run")
		// Don't return the error to prevent the entire listener from crashing
		// The provider will retry connections automatically
		return nil
	}

	return nil
}

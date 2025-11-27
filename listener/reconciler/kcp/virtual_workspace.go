package kcp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"

	"github.com/platform-mesh/golang-commons/logger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common/config"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile"
	"gopkg.in/yaml.v3"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

var (
	ErrInvalidVirtualWorkspaceURL = errors.New("invalid virtual workspace URL")
	ErrParseVirtualWorkspaceURL   = errors.New("failed to parse virtual workspace URL")
)

// VirtualWorkspace represents a virtual workspace configuration
type VirtualWorkspace struct {
	Name       string `yaml:"name"`
	URL        string `yaml:"url"`
	Kubeconfig string `yaml:"kubeconfig,omitempty"` // Optional path to kubeconfig for authentication
}

// VirtualWorkspacesConfig represents the configuration file structure
type VirtualWorkspacesConfig struct {
	VirtualWorkspaces []VirtualWorkspace `yaml:"virtualWorkspaces"`
}

// VirtualWorkspaceManager handles virtual workspace operations
type VirtualWorkspaceManager struct {
	appCfg config.Config
}

// NewVirtualWorkspaceManager creates a new virtual workspace manager
func NewVirtualWorkspaceManager(appCfg config.Config) *VirtualWorkspaceManager {
	return &VirtualWorkspaceManager{appCfg: appCfg}
}

// GetWorkspacePath returns the file path for storing the virtual workspace schema
func (v *VirtualWorkspaceManager) GetWorkspacePath(workspace VirtualWorkspace) string {
	return fmt.Sprintf("%s/%s", v.appCfg.Url.VirtualWorkspacePrefix, workspace.Name)
}

// createVirtualConfig creates a REST config for a virtual workspace
func createVirtualConfig(workspace VirtualWorkspace) (*rest.Config, error) {
	if workspace.URL == "" {
		return nil, fmt.Errorf("%w: empty URL for workspace %s", ErrInvalidVirtualWorkspaceURL, workspace.Name)
	}

	// Parse the virtual workspace URL to validate it
	_, err := url.Parse(workspace.URL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParseVirtualWorkspaceURL, err)
	}

	var virtualConfig *rest.Config

	if workspace.Kubeconfig != "" {
		// Load authentication from the specified kubeconfig
		cfg, err := clientcmd.LoadFromFile(workspace.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig %s: %w", workspace.Kubeconfig, err)
		}

		restConfig, err := clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create client config from kubeconfig %s: %w", workspace.Kubeconfig, err)
		}

		virtualConfig = restConfig
		virtualConfig.Host = workspace.URL + "/clusters/root"
	} else {
		// Use minimal configuration for virtual workspaces without authentication
		virtualConfig = &rest.Config{
			Host:      workspace.URL + "/clusters/root",
			UserAgent: "kubernetes-graphql-gateway-listener",
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: true,
			},
		}
	}

	return virtualConfig, nil
}

// CreateDiscoveryClient creates a discovery client for the virtual workspace
func (v *VirtualWorkspaceManager) CreateDiscoveryClient(workspace VirtualWorkspace) (discovery.DiscoveryInterface, error) {
	virtualConfig, err := createVirtualConfig(workspace)
	if err != nil {
		return nil, err
	}

	// Create discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(virtualConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client for virtual workspace %s (URL: %s): %w", workspace.Name, workspace.URL, err)
	}

	return discoveryClient, nil
}

// LoadConfig loads the virtual workspaces configuration from a file
func (v *VirtualWorkspaceManager) LoadConfig(configPath string) (*VirtualWorkspacesConfig, error) {
	if configPath == "" {
		return &VirtualWorkspacesConfig{}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &VirtualWorkspacesConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read virtual workspaces config file: %w", err)
	}

	var config VirtualWorkspacesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse virtual workspaces config: %w", err)
	}

	return &config, nil
}

// Virtual workspaces are now fully supported by native discovery clients
// when the URL is properly configured to include /clusters/root prefix.
// No custom wrappers needed!

// VirtualWorkspaceReconciler handles reconciliation of virtual workspaces
type VirtualWorkspaceReconciler struct {
	virtualWSManager  *VirtualWorkspaceManager
	ioHandler         *workspacefile.FileHandler
	apiSchemaResolver apischema.Resolver
	log               *logger.Logger
	mu                sync.RWMutex
	currentWorkspaces map[string]VirtualWorkspace
}

// NewVirtualWorkspaceReconciler creates a new virtual workspace reconciler
func NewVirtualWorkspaceReconciler(
	virtualWSManager *VirtualWorkspaceManager,
	ioHandler *workspacefile.FileHandler,
	apiSchemaResolver apischema.Resolver,
	log *logger.Logger,
) *VirtualWorkspaceReconciler {
	return &VirtualWorkspaceReconciler{
		virtualWSManager:  virtualWSManager,
		ioHandler:         ioHandler,
		apiSchemaResolver: apiSchemaResolver,
		log:               log,
		currentWorkspaces: make(map[string]VirtualWorkspace),
	}
}

// ReconcileConfig processes a virtual workspaces configuration update
func (r *VirtualWorkspaceReconciler) ReconcileConfig(ctx context.Context, config *VirtualWorkspacesConfig) error {
	r.log.Info().Int("count", len(config.VirtualWorkspaces)).Msg("reconciling virtual workspaces")

	// Snapshot current state under read lock to minimize contention
	r.mu.RLock()
	current := make(map[string]VirtualWorkspace, len(r.currentWorkspaces))
	for k, v := range r.currentWorkspaces {
		current[k] = v
	}
	r.mu.RUnlock()

	// Build desired map from incoming config
	desired := make(map[string]VirtualWorkspace, len(config.VirtualWorkspaces))
	for _, ws := range config.VirtualWorkspaces {
		desired[ws.Name] = ws
	}

	// Compute creations/updates (URL or kubeconfig changed) and removals
	toProcess := make([]VirtualWorkspace, 0)
	for name, ws := range desired {
		if cur, ok := current[name]; !ok || cur.URL != ws.URL || cur.Kubeconfig != ws.Kubeconfig {
			toProcess = append(toProcess, ws)
		}
	}
	toRemove := make([]string, 0)
	for name := range current {
		if _, ok := desired[name]; !ok {
			toRemove = append(toRemove, name)
		}
	}

	// Process new or updated workspaces without holding the lock
	for _, ws := range toProcess {
		r.log.Info().Str("workspace", ws.Name).Str("url", ws.URL).Msg("processing virtual workspace")
		if err := r.processVirtualWorkspace(ctx, ws); err != nil {
			r.log.Error().Err(err).Str("workspace", ws.Name).Msg("failed to process virtual workspace")
			return err
		}
	}

	// Remove deleted workspaces (best-effort, continue on error)
	for _, name := range toRemove {
		r.log.Info().Str("workspace", name).Msg("removing deleted virtual workspace")
		if err := r.removeVirtualWorkspace(name); err != nil {
			r.log.Error().Err(err).Str("workspace", name).Msg("failed to remove virtual workspace")
		}
	}

	// Update current workspaces under write lock
	r.mu.Lock()
	r.currentWorkspaces = desired
	r.mu.Unlock()

	r.log.Info().Msg("completed virtual workspaces reconciliation")
	return nil
}

// processVirtualWorkspace generates schema for a single virtual workspace
func (r *VirtualWorkspaceReconciler) processVirtualWorkspace(ctx context.Context, workspace VirtualWorkspace) error {
	workspacePath := r.virtualWSManager.GetWorkspacePath(workspace)

	r.log.Info().
		Str("workspace", workspace.Name).
		Str("url", workspace.URL).
		Str("path", workspacePath).
		Msg("generating schema for virtual workspace")

	// Create discovery client and REST mapper for the virtual workspace
	discoveryClient, restMapper, err := r.buildClientsForWorkspace(workspace)
	if err != nil {
		return err
	}

	// Use shared schema generation logic
	schemaWithMetadata, err := generateSchemaWithMetadata(
		SchemaGenerationParams{
			ClusterPath:     workspacePath,
			DiscoveryClient: discoveryClient,
			RESTMapper:      restMapper,
			HostOverride:    workspace.URL, // Use virtual workspace URL as host override
		},
		r.apiSchemaResolver,
		r.log,
	)
	if err != nil {
		return err
	}

	// Write the schema to file
	if err := r.ioHandler.Write(schemaWithMetadata, workspacePath); err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	r.log.Info().
		Str("workspace", workspace.Name).
		Str("path", workspacePath).
		Int("schemaSize", len(schemaWithMetadata)).
		Msg("successfully generated schema for virtual workspace")

	return nil
}

// removeVirtualWorkspace removes the schema file for a deleted virtual workspace
func (r *VirtualWorkspaceReconciler) removeVirtualWorkspace(name string) error {
	workspace := VirtualWorkspace{Name: name} // Create minimal workspace for path generation
	workspacePath := r.virtualWSManager.GetWorkspacePath(workspace)

	if err := r.ioHandler.Delete(workspacePath); err != nil {
		return fmt.Errorf("failed to delete schema file for workspace %s: %w", name, err)
	}

	r.log.Info().Str("workspace", name).Str("path", workspacePath).Msg("removed schema file for virtual workspace")
	return nil
}

// buildRESTMapper creates a dynamic RESTMapper for a given REST config.
func buildRESTMapper(cfg *rest.Config) (meta.RESTMapper, error) {
	httpClient, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, err
	}
	return apiutil.NewDynamicRESTMapper(cfg, httpClient)
}

// buildClientsForWorkspace consolidates creation of discovery client and REST mapper
// for a given virtual workspace. Keeps wiring in a single place.
func (r *VirtualWorkspaceReconciler) buildClientsForWorkspace(workspace VirtualWorkspace) (discovery.DiscoveryInterface, meta.RESTMapper, error) {
	// Create REST config first (single source of truth)
	virtualConfig, err := createVirtualConfig(workspace)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create REST config: %w", err)
	}

	// Discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(virtualConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	r.log.Debug().Str("workspace", workspace.Name).Str("url", workspace.URL).Msg("created discovery client for virtual workspace")

	// REST mapper
	restMapper, err := buildRESTMapper(virtualConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create REST mapper for virtual workspace: %w", err)
	}
	return discoveryClient, restMapper, nil
}

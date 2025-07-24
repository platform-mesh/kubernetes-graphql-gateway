package kcp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/openmfp/golang-commons/logger"

	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/workspacefile"
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

// CreateRESTConfig creates a REST config for the virtual workspace (for REST mappers)
func (v *VirtualWorkspaceManager) CreateRESTConfig(workspace VirtualWorkspace) (*rest.Config, error) {
	return createVirtualConfig(workspace)
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
	ioHandler         workspacefile.IOHandler
	apiSchemaResolver apischema.Resolver
	log               *logger.Logger
	mu                sync.RWMutex
	currentWorkspaces map[string]VirtualWorkspace
}

// NewVirtualWorkspaceReconciler creates a new virtual workspace reconciler
func NewVirtualWorkspaceReconciler(
	virtualWSManager *VirtualWorkspaceManager,
	ioHandler workspacefile.IOHandler,
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
	r.mu.Lock()
	defer r.mu.Unlock()

	r.log.Info().Int("count", len(config.VirtualWorkspaces)).Msg("reconciling virtual workspaces")

	// Track new workspaces for comparison
	newWorkspaces := make(map[string]VirtualWorkspace)
	for _, ws := range config.VirtualWorkspaces {
		newWorkspaces[ws.Name] = ws
	}

	// Process new or updated workspaces
	for name, workspace := range newWorkspaces {
		if current, exists := r.currentWorkspaces[name]; !exists || current.URL != workspace.URL {
			r.log.Info().Str("workspace", name).Str("url", workspace.URL).Msg("processing virtual workspace")

			if err := r.processVirtualWorkspace(ctx, workspace); err != nil {
				r.log.Error().Err(err).Str("workspace", name).Msg("failed to process virtual workspace")
				continue
			}
		}
	}

	// Remove deleted workspaces
	for name := range r.currentWorkspaces {
		if _, exists := newWorkspaces[name]; !exists {
			r.log.Info().Str("workspace", name).Msg("removing deleted virtual workspace")
			if err := r.removeVirtualWorkspace(name); err != nil {
				r.log.Error().Err(err).Str("workspace", name).Msg("failed to remove virtual workspace")
			}
		}
	}

	// Update current workspaces
	r.currentWorkspaces = newWorkspaces

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

	// Create discovery client for the virtual workspace
	discoveryClient, err := r.virtualWSManager.CreateDiscoveryClient(workspace)
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}

	r.log.Debug().Str("workspace", workspace.Name).Str("url", workspace.URL).Msg("created discovery client for virtual workspace")

	// Create REST config and mapper for the virtual workspace
	virtualConfig, err := r.virtualWSManager.CreateRESTConfig(workspace)
	if err != nil {
		return fmt.Errorf("failed to create REST config: %w", err)
	}

	httpClient, err := rest.HTTPClientFor(virtualConfig)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client for virtual workspace: %w", err)
	}

	restMapper, err := apiutil.NewDynamicRESTMapper(virtualConfig, httpClient)
	if err != nil {
		return fmt.Errorf("failed to create REST mapper for virtual workspace: %w", err)
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

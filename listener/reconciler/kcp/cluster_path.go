package kcp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	"github.com/kcp-dev/logicalcluster/v3"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/platform-mesh/golang-commons/logger"
)

var (
	ErrNilConfig             = errors.New("config cannot be nil")
	ErrNilScheme             = errors.New("scheme should not be nil")
	ErrGetClusterConfig      = errors.New("failed to get cluster config")
	ErrGetLogicalCluster     = errors.New("failed to get logicalcluster resource")
	ErrMissingPathAnnotation = errors.New("failed to get cluster path from kcp.io/path annotation")
	ErrParseHostURL          = errors.New("failed to parse rest config's Host URL")
	ErrClusterIsDeleted      = errors.New("cluster is deleted")
)

// ConfigForKCPCluster creates a rest.Config for a specific KCP cluster/workspace
// This is a shared utility used across the KCP package to avoid duplication
func ConfigForKCPCluster(clusterName string, cfg *rest.Config) (*rest.Config, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}

	// Copy the config to avoid modifying the original
	clusterCfg := rest.CopyConfig(cfg)

	// Parse the current host URL
	clusterCfgURL, err := url.Parse(clusterCfg.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host URL: %w", err)
	}

	// Set the path to point to the specific cluster/workspace
	clusterCfgURL.Path = fmt.Sprintf("/clusters/%s", clusterName)
	clusterCfg.Host = clusterCfgURL.String()

	return clusterCfg, nil
}

type ClusterPathResolver interface {
	ClientForCluster(name string) (client.Client, error)
}

type clientFactory func(config *rest.Config, options client.Options) (client.Client, error)

type ClusterPathResolverProvider struct {
	*runtime.Scheme
	*rest.Config
	clientFactory
	log *logger.Logger
}

func NewClusterPathResolver(cfg *rest.Config, scheme *runtime.Scheme, log *logger.Logger) (*ClusterPathResolverProvider, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	if scheme == nil {
		return nil, ErrNilScheme
	}
	if log == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	return &ClusterPathResolverProvider{
		Scheme:        scheme,
		Config:        cfg,
		clientFactory: client.New,
		log:           log,
	}, nil
}

func (cp *ClusterPathResolverProvider) ClientForCluster(name string) (client.Client, error) {
	clusterConfig, err := ConfigForKCPCluster(name, cp.Config)
	if err != nil {
		return nil, errors.Join(ErrGetClusterConfig, err)
	}
	return cp.clientFactory(clusterConfig, client.Options{Scheme: cp.Scheme})
}

func (cp *ClusterPathResolverProvider) PathForCluster(name string, clt client.Client) (string, error) {
	if name == "root" {
		return name, nil
	}

	// Try to get LogicalCluster resource to extract workspace path
	lc := &kcpcore.LogicalCluster{}
	err := clt.Get(context.TODO(), client.ObjectKey{Name: "cluster"}, lc)
	if err != nil {
		cp.log.Debug().
			Err(err).
			Str("clusterName", name).
			Msg("LogicalCluster resource not accessible, using cluster name as fallback")
		return name, nil
	}

	if lc.DeletionTimestamp != nil {
		// Try to get the workspace name even if the cluster is being deleted
		// First try the kcp.io/path annotation (most reliable)
		if lc.Annotations != nil {
			if path, ok := lc.Annotations["kcp.io/path"]; ok {
				return path, ErrClusterIsDeleted
			}
		}
		// Fallback to logicalcluster.From()
		workspaceName := logicalcluster.From(lc).String()
		if workspaceName != "" {
			return workspaceName, ErrClusterIsDeleted
		}
		return name, ErrClusterIsDeleted
	}

	// Primary approach: Extract the workspace path from the kcp.io/path annotation
	// This is the most reliable method as proven by our debug script
	if lc.Annotations != nil {
		if path, ok := lc.Annotations["kcp.io/path"]; ok {
			return path, nil
		}
	}

	// Fallback: Use logicalcluster.From() to get the actual workspace name
	// This is the same approach used by the virtual-workspaces resolver
	workspaceName := logicalcluster.From(lc).String()
	if workspaceName != "" {
		return workspaceName, nil
	}

	// Final fallback: use cluster name as-is
	return name, nil
}

// PathForClusterFromConfig attempts to extract cluster identifier from cluster configuration
// Returns either a workspace path or cluster hash depending on the URL type.
// This is an alternative approach when LogicalCluster resource is not accessible.
func PathForClusterFromConfig(clusterName string, cfg *rest.Config) (string, error) {
	if clusterName == "root" {
		return clusterName, nil
	}

	if cfg == nil {
		return clusterName, nil
	}

	// Parse the cluster config host URL to extract workspace information
	parsedURL, err := url.Parse(cfg.Host)
	if err != nil {
		return clusterName, nil
	}

	// Check if the URL path contains cluster information
	// KCP URLs typically follow patterns like:
	// - /clusters/{workspace-path} for direct cluster access (e.g., /clusters/root:orgs:default)
	// - /services/apiexport/{workspace-path}/{export-name} for virtual workspaces
	if strings.HasPrefix(parsedURL.Path, "/clusters/") {
		// Extract workspace path from URL path
		pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
		if len(pathParts) >= 2 && pathParts[0] == "clusters" {
			// The cluster name in the URL should be the workspace path
			urlWorkspacePath := pathParts[1]
			// If the URL workspace path looks like a workspace path (contains colons), use it
			if strings.Contains(urlWorkspacePath, ":") {
				return urlWorkspacePath, nil
			}
			// Even if it doesn't contain colons, it might still be a valid workspace path
			// (e.g., "root" or single-level workspaces)
			if urlWorkspacePath != clusterName {
				return urlWorkspacePath, nil
			}
		}
	}

	// Check for virtual workspace patterns
	if strings.HasPrefix(parsedURL.Path, "/services/apiexport/") {
		// Pattern: /services/apiexport/{workspace-path}/{export-name}
		workspacePath, _, err := extractAPIExportRef(parsedURL.String())
		if err != nil {
			// Return error if we can't parse the APIExport URL properly
			return "", fmt.Errorf("failed to parse APIExport URL: %w", err)
		}
		// Return the workspace path from the parsed APIExport URL
		return workspacePath, nil
	}

	// If we can't extract meaningful cluster identifier, fall back to cluster name
	return clusterName, nil
}

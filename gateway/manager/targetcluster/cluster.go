package targetcluster

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-openapi/spec"
	"github.com/openmfp/golang-commons/logger"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/kcp"

	"github.com/openmfp/kubernetes-graphql-gateway/common/auth"
	appConfig "github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/schema"
)

// FileData represents the data extracted from a schema file
type FileData struct {
	Definitions     map[string]any   `json:"definitions"`
	ClusterMetadata *ClusterMetadata `json:"x-cluster-metadata,omitempty"`
}

// ClusterMetadata represents the cluster connection metadata stored in schema files
type ClusterMetadata struct {
	Host string        `json:"host"`
	Path string        `json:"path,omitempty"`
	Auth *AuthMetadata `json:"auth,omitempty"`
	CA   *CAMetadata   `json:"ca,omitempty"`
}

// AuthMetadata represents authentication information
type AuthMetadata struct {
	Type       string `json:"type"`
	Token      string `json:"token,omitempty"`
	Kubeconfig string `json:"kubeconfig,omitempty"`
	CertData   string `json:"certData,omitempty"`
	KeyData    string `json:"keyData,omitempty"`
}

// CAMetadata represents CA certificate information
type CAMetadata struct {
	Data string `json:"data"`
}

// TargetCluster represents a single target Kubernetes cluster
type TargetCluster struct {
	appCfg        appConfig.Config
	name          string
	client        client.WithWatch
	restCfg       *rest.Config
	handler       *GraphQLHandler
	graphqlServer *GraphQLServer
	log           *logger.Logger
}

// NewTargetCluster creates a new TargetCluster from a schema file
func NewTargetCluster(
	name string,
	schemaFilePath string,
	log *logger.Logger,
	appCfg appConfig.Config,
	roundTripperFactory func(http.RoundTripper, rest.TLSClientConfig) http.RoundTripper,
) (*TargetCluster, error) {
	fileData, err := readSchemaFile(schemaFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	cluster := &TargetCluster{
		appCfg: appCfg,
		name:   name,
		log:    log,
	}

	// Connect to cluster - use metadata if available, otherwise fall back to standard config
	if err := cluster.connect(appCfg, fileData.ClusterMetadata, roundTripperFactory); err != nil {
		return nil, fmt.Errorf("failed to connect to cluster: %w", err)
	}

	// Create GraphQL schema and handler
	if err := cluster.createHandler(fileData.Definitions, appCfg); err != nil {
		return nil, fmt.Errorf("failed to create GraphQL handler: %w", err)
	}

	log.Info().
		Str("cluster", name).
		Str("endpoint", cluster.GetEndpoint(appCfg)).
		Msg("Registered endpoint")

	return cluster, nil
}

// connect establishes connection to the target cluster
func (tc *TargetCluster) connect(appCfg appConfig.Config, metadata *ClusterMetadata, roundTripperFactory func(http.RoundTripper, rest.TLSClientConfig) http.RoundTripper) error {
	// All clusters now use metadata from schema files to get kubeconfig
	if metadata == nil {
		return fmt.Errorf("cluster %s requires cluster metadata in schema file", tc.name)
	}

	tc.log.Info().
		Str("cluster", tc.name).
		Str("host", metadata.Host).
		Bool("isVirtualWorkspace", strings.HasPrefix(tc.name, tc.appCfg.Url.VirtualWorkspacePrefix)).
		Msg("Using cluster metadata from schema file for connection")

	var err error
	tc.restCfg, err = buildConfigFromMetadata(metadata, tc.log)
	if err != nil {
		return fmt.Errorf("failed to build config from metadata: %w", err)
	}

	if roundTripperFactory != nil {
		tc.restCfg.Wrap(func(rt http.RoundTripper) http.RoundTripper {
			return roundTripperFactory(rt, tc.restCfg.TLSClientConfig)
		})
	}

	// Create client - use KCP-aware client only for KCP mode, standard client otherwise
	if appCfg.EnableKcp {
		tc.client, err = kcp.NewClusterAwareClientWithWatch(tc.restCfg, client.Options{})
	} else {
		tc.client, err = client.NewWithWatch(tc.restCfg, client.Options{})
	}
	if err != nil {
		return fmt.Errorf("failed to create cluster client: %w", err)
	}

	return nil
}

// buildConfigFromMetadata creates rest.Config from cluster metadata
func buildConfigFromMetadata(metadata *ClusterMetadata, log *logger.Logger) (*rest.Config, error) {
	var authType, token, kubeconfig, certData, keyData, caData string

	if metadata.Auth != nil {
		authType = metadata.Auth.Type
		token = metadata.Auth.Token
		kubeconfig = metadata.Auth.Kubeconfig
		certData = metadata.Auth.CertData
		keyData = metadata.Auth.KeyData
	}

	if metadata.CA != nil {
		caData = metadata.CA.Data
	}

	// Use common auth package
	config, err := auth.BuildConfigFromMetadata(metadata.Host, authType, token, kubeconfig, certData, keyData, caData)
	if err != nil {
		return nil, err
	}

	log.Debug().
		Str("host", metadata.Host).
		Str("authType", authType).
		Bool("hasCA", caData != "").
		Msg("configured cluster from metadata")

	return config, nil
}

// createHandler creates the GraphQL schema and handler
func (tc *TargetCluster) createHandler(definitions map[string]interface{}, appCfg appConfig.Config) error {
	// Convert definitions to spec format
	specDefs, err := convertToSpecDefinitions(definitions)
	if err != nil {
		return fmt.Errorf("failed to convert definitions: %w", err)
	}

	// Create resolver
	resolverProvider := resolver.New(tc.log, tc.client)

	// Create schema gateway
	schemaGateway, err := schema.New(tc.log, specDefs, resolverProvider)
	if err != nil {
		return fmt.Errorf("failed to create GraphQL schema: %w", err)
	}

	// Create and store GraphQL server and handler
	tc.graphqlServer = NewGraphQLServer(tc.log, appCfg)
	tc.handler = tc.graphqlServer.CreateHandler(schemaGateway.GetSchema())

	return nil
}

// GetName returns the cluster name
func (tc *TargetCluster) GetName() string {
	return tc.name
}

// GetConfig returns the cluster's rest.Config
func (tc *TargetCluster) GetConfig() *rest.Config {
	return tc.restCfg
}

// GetEndpoint returns the HTTP endpoint for this cluster's GraphQL API
func (tc *TargetCluster) GetEndpoint(appCfg appConfig.Config) string {
	// Build the path with virtual workspace suffix if needed
	// tc.name format:
	// - For virtual workspaces: "virtual-workspace/{name}"
	// - For regular workspaces: "{workspace-name}"
	path := tc.name
	if strings.HasPrefix(path, appCfg.Url.VirtualWorkspacePrefix) {
		path = fmt.Sprintf("%s/%s", path, appCfg.Url.DefaultKcpWorkspace)
	}

	if appCfg.LocalDevelopment {
		return fmt.Sprintf("http://localhost:%s/%s/%s", appCfg.Gateway.Port, path, appCfg.Url.GraphqlSuffix)
	}

	return fmt.Sprintf("/%s/%s", path, appCfg.Url.GraphqlSuffix)
}

// ServeHTTP handles HTTP requests for this cluster
func (tc *TargetCluster) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if tc.handler == nil || tc.handler.Handler == nil {
		http.Error(w, "Cluster not ready", http.StatusServiceUnavailable)
		return
	}

	// Handle subscription requests using Server-Sent Events
	if r.Header.Get("Accept") == "text/event-stream" {
		tc.graphqlServer.HandleSubscription(w, r, tc.handler.Schema)
		return
	}

	tc.handler.Handler.ServeHTTP(w, r)
}

// readSchemaFile reads and parses a schema file
func readSchemaFile(filePath string) (*FileData, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var fileData FileData
	if err := json.Unmarshal(data, &fileData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &fileData, nil
}

// convertToSpecDefinitions converts map definitions to go-openapi spec format
func convertToSpecDefinitions(definitions map[string]interface{}) (spec.Definitions, error) {
	data, err := json.Marshal(definitions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal definitions: %w", err)
	}

	var specDefs spec.Definitions
	if err := json.Unmarshal(data, &specDefs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to spec definitions: %w", err)
	}

	return specDefs, nil
}

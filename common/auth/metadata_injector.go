package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmfp/golang-commons/logger"
	gatewayv1alpha1 "github.com/openmfp/kubernetes-graphql-gateway/common/apis/v1alpha1"
)

// MetadataInjectionConfig contains configuration for metadata injection
type MetadataInjectionConfig struct {
	Host         string
	Path         string
	Auth         *gatewayv1alpha1.AuthConfig
	CA           *gatewayv1alpha1.CAConfig
	HostOverride string // For virtual workspaces
}

// InjectClusterMetadata injects cluster metadata into schema JSON
// This unified function handles both KCP and ClusterAccess use cases
func InjectClusterMetadata(ctx context.Context, schemaJSON []byte, config MetadataInjectionConfig, k8sClient client.Client, log *logger.Logger) ([]byte, error) {
	// Parse the existing schema JSON
	var schemaData map[string]interface{}
	if err := json.Unmarshal(schemaJSON, &schemaData); err != nil {
		return nil, fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	// Determine the host to use
	host := determineHost(config.Host, config.HostOverride, log)

	// Create cluster metadata
	metadata := map[string]interface{}{
		"host": host,
		"path": config.Path,
	}

	// Add auth data if configured
	if config.Auth != nil {
		authMetadata, err := extractAuthDataForMetadata(ctx, config.Auth, k8sClient)
		if err != nil {
			log.Warn().Err(err).Msg("failed to extract auth data for metadata")
		} else if authMetadata != nil {
			metadata["auth"] = authMetadata
		}
	}

	// Add CA data - prefer explicit CA config, fallback to kubeconfig CA
	if config.CA != nil {
		caData, err := ExtractCAData(ctx, config.CA, k8sClient)
		if err != nil {
			log.Warn().Err(err).Msg("failed to extract CA data for metadata")
		} else if caData != nil {
			metadata["ca"] = map[string]interface{}{
				"data": base64.StdEncoding.EncodeToString(caData),
			}
		}
	} else if config.Auth != nil {
		tryExtractKubeconfigCA(ctx, config.Auth, k8sClient, metadata, log)
	}

	return finalizeSchemaInjection(schemaData, metadata, host, config.Path, config.CA != nil || config.Auth != nil, log)
}

// InjectKCPMetadataFromEnv injects KCP metadata using kubeconfig from environment
// This is a convenience function for KCP use cases
func InjectKCPMetadataFromEnv(schemaJSON []byte, clusterPath string, log *logger.Logger, hostOverride ...string) ([]byte, error) {
	// Get kubeconfig from environment (same sources as ctrl.GetConfig())
	kubeconfigData, kubeconfigHost, err := extractKubeconfigFromEnv(log)
	if err != nil {
		return nil, fmt.Errorf("failed to extract kubeconfig data: %w", err)
	}

	// Determine host override
	var override string
	if len(hostOverride) > 0 && hostOverride[0] != "" {
		override = hostOverride[0]
	}

	// Parse the existing schema JSON
	var schemaData map[string]interface{}
	if err := json.Unmarshal(schemaJSON, &schemaData); err != nil {
		return nil, fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	// Determine which host to use
	host := determineKCPHost(kubeconfigHost, override, clusterPath, log)

	// Create cluster metadata with environment kubeconfig
	metadata := map[string]interface{}{
		"host": host,
		"path": clusterPath,
		"auth": map[string]interface{}{
			"type":       "kubeconfig",
			"kubeconfig": base64.StdEncoding.EncodeToString(kubeconfigData),
		},
	}

	// Extract CA data from kubeconfig if available
	caData := extractCAFromKubeconfigData(kubeconfigData, log)
	if caData != nil {
		metadata["ca"] = map[string]interface{}{
			"data": base64.StdEncoding.EncodeToString(caData),
		}
	}

	return finalizeSchemaInjection(schemaData, metadata, host, clusterPath, caData != nil, log)
}

// extractAuthDataForMetadata extracts auth data from AuthConfig for metadata injection
func extractAuthDataForMetadata(ctx context.Context, auth *gatewayv1alpha1.AuthConfig, k8sClient client.Client) (map[string]interface{}, error) {
	if auth == nil {
		return nil, nil
	}

	if auth.SecretRef != nil {
		return extractTokenAuth(ctx, auth.SecretRef, k8sClient)
	}

	if auth.KubeconfigSecretRef != nil {
		return extractKubeconfigAuth(ctx, auth.KubeconfigSecretRef, k8sClient)
	}

	if auth.ClientCertificateRef != nil {
		return extractClientCertAuth(ctx, auth.ClientCertificateRef, k8sClient)
	}

	return nil, nil // No auth configured
}

// extractTokenAuth handles token-based authentication from SecretRef
func extractTokenAuth(ctx context.Context, secretRef *gatewayv1alpha1.SecretRef, k8sClient client.Client) (map[string]interface{}, error) {
	secret, err := getSecret(ctx, secretRef.Name, secretRef.Namespace, k8sClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth secret: %w", err)
	}

	tokenData, ok := secret.Data[secretRef.Key]
	if !ok {
		return nil, fmt.Errorf("auth key not found in secret")
	}

	return map[string]interface{}{
		"type":  "token",
		"token": base64.StdEncoding.EncodeToString(tokenData),
	}, nil
}

// extractKubeconfigAuth handles kubeconfig-based authentication from KubeconfigSecretRef
func extractKubeconfigAuth(ctx context.Context, kubeconfigRef *gatewayv1alpha1.KubeconfigSecretRef, k8sClient client.Client) (map[string]interface{}, error) {
	secret, err := getSecret(ctx, kubeconfigRef.Name, kubeconfigRef.Namespace, k8sClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig secret: %w", err)
	}

	kubeconfigData, ok := secret.Data["kubeconfig"]
	if !ok {
		return nil, fmt.Errorf("kubeconfig key not found in secret")
	}

	return map[string]interface{}{
		"type":       "kubeconfig",
		"kubeconfig": base64.StdEncoding.EncodeToString(kubeconfigData),
	}, nil
}

// extractClientCertAuth handles client certificate authentication from ClientCertificateRef
func extractClientCertAuth(ctx context.Context, certRef *gatewayv1alpha1.ClientCertificateRef, k8sClient client.Client) (map[string]interface{}, error) {
	secret, err := getSecret(ctx, certRef.Name, certRef.Namespace, k8sClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get client certificate secret: %w", err)
	}

	certData, certOk := secret.Data["tls.crt"]
	keyData, keyOk := secret.Data["tls.key"]

	if !certOk || !keyOk {
		return nil, fmt.Errorf("client certificate or key not found in secret")
	}

	return map[string]interface{}{
		"type":     "clientCert",
		"certData": base64.StdEncoding.EncodeToString(certData),
		"keyData":  base64.StdEncoding.EncodeToString(keyData),
	}, nil
}

// getSecret is a helper function to retrieve secrets with namespace defaulting
func getSecret(ctx context.Context, name, namespace string, k8sClient client.Client) (*corev1.Secret, error) {
	if namespace == "" {
		namespace = "default"
	}

	secret := &corev1.Secret{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

// extractKubeconfigFromEnv gets kubeconfig data from the same sources as ctrl.GetConfig()
func extractKubeconfigFromEnv(log *logger.Logger) ([]byte, string, error) {
	// Check KUBECONFIG environment variable first
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath != "" {
		log.Debug().Str("source", "KUBECONFIG env var").Str("path", kubeconfigPath).Msg("using kubeconfig from environment variable")
	}

	// Fall back to default kubeconfig location if not set
	if kubeconfigPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, "", fmt.Errorf("failed to determine kubeconfig location: %w", err)
		}
		kubeconfigPath = home + "/.kube/config"
		log.Debug().Str("source", "default location").Str("path", kubeconfigPath).Msg("using default kubeconfig location")
	}

	// Check if file exists
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("kubeconfig file not found: %s", kubeconfigPath)
	}

	// Read kubeconfig file
	kubeconfigData, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read kubeconfig file %s: %w", kubeconfigPath, err)
	}

	// Parse kubeconfig to extract server URL
	config, err := clientcmd.Load(kubeconfigData)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	// Get current context and cluster server URL
	host, err := extractServerURL(config)
	if err != nil {
		return nil, "", fmt.Errorf("failed to extract server URL from kubeconfig: %w", err)
	}

	return kubeconfigData, host, nil
}

// extractServerURL extracts the server URL from kubeconfig
func extractServerURL(config *api.Config) (string, error) {
	if config.CurrentContext == "" {
		return "", fmt.Errorf("no current context in kubeconfig")
	}

	context, exists := config.Contexts[config.CurrentContext]
	if !exists {
		return "", fmt.Errorf("current context %s not found in kubeconfig", config.CurrentContext)
	}

	cluster, exists := config.Clusters[context.Cluster]
	if !exists {
		return "", fmt.Errorf("cluster %s not found in kubeconfig", context.Cluster)
	}

	if cluster.Server == "" {
		return "", fmt.Errorf("no server URL found in cluster configuration")
	}

	return cluster.Server, nil
}

// stripVirtualWorkspacePath removes virtual workspace paths from a URL to get the base KCP host
func stripVirtualWorkspacePath(hostURL string) string {
	parsedURL, err := url.Parse(hostURL)
	if err != nil {
		// If we can't parse the URL, return it as-is
		return hostURL
	}

	// Check if the path contains a virtual workspace pattern: /services/apiexport/...
	if strings.HasPrefix(parsedURL.Path, "/services/apiexport/") {
		// Strip the virtual workspace path to get the base KCP host
		parsedURL.Path = ""
		return parsedURL.String()
	}

	// If it's not a virtual workspace URL, return as-is
	return hostURL
}

// extractCAFromKubeconfigData extracts CA certificate data from raw kubeconfig bytes
func extractCAFromKubeconfigData(kubeconfigData []byte, log *logger.Logger) []byte {
	config, err := clientcmd.Load(kubeconfigData)
	if err != nil {
		log.Warn().Err(err).Msg("failed to parse kubeconfig for CA extraction")
		return nil
	}

	if config.CurrentContext == "" {
		log.Warn().Msg("no current context in kubeconfig for CA extraction")
		return nil
	}

	context, exists := config.Contexts[config.CurrentContext]
	if !exists {
		log.Warn().Str("context", config.CurrentContext).Msg("current context not found in kubeconfig for CA extraction")
		return nil
	}

	cluster, exists := config.Clusters[context.Cluster]
	if !exists {
		log.Warn().Str("cluster", context.Cluster).Msg("cluster not found in kubeconfig for CA extraction")
		return nil
	}

	if len(cluster.CertificateAuthorityData) == 0 {
		log.Debug().Msg("no CA data found in kubeconfig")
		return nil
	}

	return cluster.CertificateAuthorityData
}

// extractCAFromKubeconfigB64 extracts CA certificate data from base64-encoded kubeconfig
func extractCAFromKubeconfigB64(kubeconfigB64 string, log *logger.Logger) []byte {
	kubeconfigData, err := base64.StdEncoding.DecodeString(kubeconfigB64)
	if err != nil {
		log.Warn().Err(err).Msg("failed to decode kubeconfig for CA extraction")
		return nil
	}

	return extractCAFromKubeconfigData(kubeconfigData, log)
}

// tryExtractKubeconfigCA attempts to extract CA data from kubeconfig auth and adds it to metadata
func tryExtractKubeconfigCA(ctx context.Context, auth *gatewayv1alpha1.AuthConfig, k8sClient client.Client, metadata map[string]interface{}, log *logger.Logger) {
	authMetadata, err := extractAuthDataForMetadata(ctx, auth, k8sClient)
	if err != nil {
		log.Warn().Err(err).Msg("failed to extract auth data for CA extraction")
		return
	}

	if authMetadata == nil {
		return
	}

	authType, ok := authMetadata["type"].(string)
	if !ok || authType != "kubeconfig" {
		return
	}

	kubeconfigB64, ok := authMetadata["kubeconfig"].(string)
	if !ok {
		return
	}

	kubeconfigCAData := extractCAFromKubeconfigB64(kubeconfigB64, log)
	if kubeconfigCAData == nil {
		return
	}

	metadata["ca"] = map[string]interface{}{
		"data": base64.StdEncoding.EncodeToString(kubeconfigCAData),
	}
	log.Info().Msg("extracted CA data from kubeconfig")
}

// determineHost determines which host to use based on configuration
func determineHost(originalHost, hostOverride string, log *logger.Logger) string {
	if hostOverride != "" {
		log.Info().
			Str("originalHost", originalHost).
			Str("overrideHost", hostOverride).
			Msg("using host override for virtual workspace")
		return hostOverride
	}

	// For normal workspaces, ensure we use a clean host by stripping any virtual workspace paths
	cleanedHost := stripVirtualWorkspacePath(originalHost)
	if cleanedHost != originalHost {
		log.Info().
			Str("originalHost", originalHost).
			Str("cleanedHost", cleanedHost).
			Msg("cleaned virtual workspace path from host for normal workspace")
	}
	return cleanedHost
}

// determineKCPHost determines which host to use for KCP metadata injection
func determineKCPHost(kubeconfigHost, override, clusterPath string, log *logger.Logger) string {
	if override != "" {
		log.Info().
			Str("clusterPath", clusterPath).
			Str("originalHost", kubeconfigHost).
			Str("overrideHost", override).
			Msg("using host override for virtual workspace")
		return override
	}

	// For normal workspaces, ensure we use a clean KCP host by stripping any virtual workspace paths
	host := stripVirtualWorkspacePath(kubeconfigHost)
	if host != kubeconfigHost {
		log.Info().
			Str("clusterPath", clusterPath).
			Str("originalHost", kubeconfigHost).
			Str("cleanedHost", host).
			Msg("cleaned virtual workspace path from kubeconfig host for normal workspace")
	}
	return host
}

// finalizeSchemaInjection finalizes the schema injection process
func finalizeSchemaInjection(schemaData map[string]interface{}, metadata map[string]interface{}, host, path string, hasCA bool, log *logger.Logger) ([]byte, error) {
	// Inject the metadata into the schema
	schemaData["x-cluster-metadata"] = metadata

	// Marshal back to JSON
	modifiedJSON, err := json.Marshal(schemaData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified schema: %w", err)
	}

	log.Info().
		Str("host", host).
		Str("path", path).
		Bool("hasCA", hasCA).
		Msg("successfully injected cluster metadata into schema")

	return modifiedJSON, nil
}

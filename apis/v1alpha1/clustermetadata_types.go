package v1alpha1

import (
	"encoding/base64"
	"errors"
	"fmt"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kube-openapi/pkg/spec3"
)

// Schema represents the data extracted from a schema file
type Schema struct {
	Components      *spec3.Components `json:"components,omitempty"`
	ClusterMetadata *ClusterMetadata  `json:"x-cluster-metadata,omitempty"`
}

// ClusterMetadataFunc is a function type that returns ClusterMetadata for a given cluster name
type ClusterMetadataFunc func(clusterName string) (*ClusterMetadata, error)

// ClusterURLResolver is function that will resolve cluster url for a given cluster name
type ClusterURLResolver func(currentURL, clusterName string) (string, error)

// DefaultClusterURLResolverFunc is the default implementation that returns the URL unchanged
func DefaultClusterURLResolverFunc(url, clusterName string) (string, error) {
	return url, nil
}

// These following types are used to store cluster connection metadata in schema files
// They are not used directly in Kubernetes resources.

// ClusterMetadata represents the cluster connection metadata stored in schema files.
type ClusterMetadata struct {
	Host string        `json:"host"`
	Path string        `json:"path,omitempty"`
	Auth *AuthMetadata `json:"auth,omitempty"`
	CA   *CAMetadata   `json:"ca,omitempty"`
}

type AuthenticationType string

const (
	AuthTypeToken      AuthenticationType = "token"
	AuthTypeKubeconfig AuthenticationType = "kubeconfig"
	AuthTypeClientCert AuthenticationType = "clientCert"
)

// AuthMetadata represents authentication information
type AuthMetadata struct {
	Type       AuthenticationType `json:"type"`
	Token      string             `json:"token,omitempty"`
	Kubeconfig string             `json:"kubeconfig,omitempty"`
	CertData   string             `json:"certData,omitempty"`
	KeyData    string             `json:"keyData,omitempty"`
}

// CAMetadata represents CA certificate information
type CAMetadata struct {
	Data string `json:"data"`
}

// buildConfigFromMetadata creates a rest.Config from base64-encoded metadata (used by gateway)
func BuildRestConfigFromMetadata(metadata ClusterMetadata) (*rest.Config, error) {
	return buildConfigFromMetadata(metadata)
}

// BuildRestConfigFromClusterAccess creates a rest.Config from base64-encoded metadata (used by gateway)
func BuildRestConfigFromClusterAccess(ca ClusterAccess) (*rest.Config, error) {
	return buildConfigFromClusterAccess(ca)
}

// buildConfigFromMetadata creates a rest.Config from base64-encoded metadata (used by gateway)
func BuildClusterMetadataFromClusterAccess(ca ClusterAccess) (*ClusterMetadata, error) {
	return buildClusterMetadataFromClusterAccess(ca)
}

// buildConfigFromClusterAccess builds ClusterMetadata from ClusterAccess
func buildClusterMetadataFromClusterAccess(ca ClusterAccess) (*ClusterMetadata, error) {
	// TODO: Implement
	return nil, errors.New("not implemented")
}

// buildConfigFromClusterAccess creates a rest.Config from ClusterAccess
func buildConfigFromClusterAccess(ca ClusterAccess) (*rest.Config, error) {
	metadata, err := buildClusterMetadataFromClusterAccess(ca)
	if err != nil {
		return nil, err
	}
	return buildConfigFromMetadata(*metadata)
}

// buildConfigFromMetadata creates a rest.Config from base64-encoded metadata (used by gateway)
func buildConfigFromMetadata(metadata ClusterMetadata) (*rest.Config, error) {
	if metadata.Host == "" {
		return nil, errors.New("host is required")
	}

	config := &rest.Config{
		Host: metadata.Host,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true, // Start with insecure, will be overridden if CA is provided
		},
	}

	// Handle CA data
	if metadata.CA != nil && metadata.CA.Data != "" {
		decodedCA, err := base64.StdEncoding.DecodeString(metadata.CA.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to decode CA data: %w", err)
		}
		config.CAData = decodedCA
		config.Insecure = false
	}

	// Handle authentication based on type if we have it
	if metadata.Auth == nil {
		return config, nil
	}
	switch metadata.Auth.Type {
	case AuthTypeToken:
		if metadata.Auth.Token != "" {
			tokenData, err := base64.StdEncoding.DecodeString(metadata.Auth.Token)
			if err != nil {
				return nil, fmt.Errorf("failed to decode token: %w", err)
			}
			config.BearerToken = string(tokenData)
		}
	case AuthTypeKubeconfig:
		if metadata.Auth.Kubeconfig != "" {
			kubeconfigData, err := base64.StdEncoding.DecodeString(metadata.Auth.Kubeconfig)
			if err != nil {
				return nil, fmt.Errorf("failed to decode kubeconfig: %w", err)
			}

			if err := configureFromKubeconfig(config, kubeconfigData); err != nil {
				return nil, fmt.Errorf("failed to configure from kubeconfig: %w", err)
			}
		}
	case AuthTypeClientCert:
		if metadata.Auth.CertData != "" && metadata.Auth.KeyData != "" {
			decodedCert, err := base64.StdEncoding.DecodeString(metadata.Auth.CertData)
			if err != nil {
				return nil, fmt.Errorf("failed to decode cert data: %w", err)
			}
			decodedKey, err := base64.StdEncoding.DecodeString(metadata.Auth.KeyData)
			if err != nil {
				return nil, fmt.Errorf("failed to decode key data: %w", err)
			}
			config.CertData = decodedCert
			config.KeyData = decodedKey
		}
	}

	return config, nil
}

// configureFromKubeconfig configures authentication from kubeconfig data
func configureFromKubeconfig(config *rest.Config, kubeconfigData []byte) error {
	// Parse kubeconfig and extract auth info
	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfigData)
	if err != nil {
		return errors.Join(errors.New("failed to parse kubeconfig"), err)
	}

	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return errors.Join(errors.New("failed to get raw kubeconfig"), err)
	}

	// Get the current context
	currentContext := rawConfig.CurrentContext
	if currentContext == "" {
		return errors.New("no current context in kubeconfig")
	}

	context, exists := rawConfig.Contexts[currentContext]
	if !exists {
		return errors.New("current context not found in kubeconfig")
	}

	// Get auth info for current context
	authInfo, exists := rawConfig.AuthInfos[context.AuthInfo]
	if !exists {
		return errors.New("auth info not found in kubeconfig")
	}

	return extractAuthFromKubeconfig(config, authInfo)
}

// extractAuthFromKubeconfig extracts authentication info from kubeconfig AuthInfo
func extractAuthFromKubeconfig(config *rest.Config, authInfo *api.AuthInfo) error {
	if authInfo.Token != "" {
		config.BearerToken = authInfo.Token
		return nil
	}

	if authInfo.TokenFile != "" {
		// TODO: Read token from file if needed
		return errors.New("token file authentication not yet implemented")
	}

	if len(authInfo.ClientCertificateData) > 0 && len(authInfo.ClientKeyData) > 0 {
		config.CertData = authInfo.ClientCertificateData
		config.KeyData = authInfo.ClientKeyData
		return nil
	}

	if authInfo.ClientCertificate != "" && authInfo.ClientKey != "" {
		config.CertFile = authInfo.ClientCertificate
		config.KeyFile = authInfo.ClientKey
		return nil
	}

	if authInfo.Username != "" && authInfo.Password != "" {
		config.Username = authInfo.Username
		config.Password = authInfo.Password
		return nil
	}

	// No recognizable authentication found
	return errors.New("no valid authentication method found in kubeconfig")
}

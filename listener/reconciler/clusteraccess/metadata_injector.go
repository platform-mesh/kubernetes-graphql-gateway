package clusteraccess

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmfp/golang-commons/logger"
	gatewayv1alpha1 "github.com/openmfp/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/openmfp/kubernetes-graphql-gateway/common/auth"
)

func injectClusterMetadata(schemaJSON []byte, clusterAccess gatewayv1alpha1.ClusterAccess, k8sClient client.Client, log *logger.Logger) ([]byte, error) {
	// Parse the existing schema JSON
	var schemaData map[string]interface{}
	if err := json.Unmarshal(schemaJSON, &schemaData); err != nil {
		return nil, fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	// Create cluster metadata
	metadata := map[string]interface{}{
		"host": clusterAccess.Spec.Host,
	}

	// Add path if specified
	if clusterAccess.Spec.Path != "" {
		metadata["path"] = clusterAccess.Spec.Path
	} else {
		metadata["path"] = clusterAccess.GetName()
	}

	// Extract auth data and potentially CA data from kubeconfig
	var kubeconfigCAData []byte
	if clusterAccess.Spec.Auth != nil {
		authMetadata, err := extractAuthDataForMetadata(clusterAccess.Spec.Auth, k8sClient)
		if err != nil {
			log.Warn().Err(err).Str("clusterAccess", clusterAccess.GetName()).Msg("failed to extract auth data for metadata")
		} else if authMetadata != nil {
			metadata["auth"] = authMetadata

			// If auth type is kubeconfig, extract CA data from kubeconfig
			if authType, ok := authMetadata["type"].(string); ok && authType == "kubeconfig" {
				if kubeconfigB64, ok := authMetadata["kubeconfig"].(string); ok {
					kubeconfigCAData = extractCAFromKubeconfig(kubeconfigB64, log)
				}
			}
		}
	}

	// Add CA data - prefer explicit CA config, fallback to kubeconfig CA
	if clusterAccess.Spec.CA != nil {
		caData, err := extractCADataForMetadata(clusterAccess.Spec.CA, k8sClient)
		if err != nil {
			log.Warn().Err(err).Str("clusterAccess", clusterAccess.GetName()).Msg("failed to extract CA data for metadata")
		} else if caData != nil {
			metadata["ca"] = map[string]interface{}{
				"data": base64.StdEncoding.EncodeToString(caData),
			}
		}
	} else if kubeconfigCAData != nil {
		// Use CA data extracted from kubeconfig
		metadata["ca"] = map[string]interface{}{
			"data": base64.StdEncoding.EncodeToString(kubeconfigCAData),
		}
		log.Info().Str("clusterAccess", clusterAccess.GetName()).Msg("extracted CA data from kubeconfig")
	}

	// Inject the metadata into the schema
	schemaData["x-cluster-metadata"] = metadata

	// Marshal back to JSON
	modifiedJSON, err := json.Marshal(schemaData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified schema: %w", err)
	}

	log.Info().
		Str("clusterAccess", clusterAccess.GetName()).
		Str("host", clusterAccess.Spec.Host).
		Msg("successfully injected cluster metadata into schema")

	return modifiedJSON, nil
}

func extractCADataForMetadata(ca *gatewayv1alpha1.CAConfig, k8sClient client.Client) ([]byte, error) {
	return auth.ExtractCAData(ca, k8sClient)
}

func extractAuthDataForMetadata(auth *gatewayv1alpha1.AuthConfig, k8sClient client.Client) (map[string]interface{}, error) {
	if auth == nil {
		return nil, nil
	}

	ctx := context.Background()

	if auth.SecretRef != nil {
		secret := &corev1.Secret{}
		namespace := auth.SecretRef.Namespace
		if namespace == "" {
			namespace = "default"
		}

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      auth.SecretRef.Name,
			Namespace: namespace,
		}, secret)
		if err != nil {
			return nil, fmt.Errorf("failed to get auth secret: %w", err)
		}

		tokenData, ok := secret.Data[auth.SecretRef.Key]
		if !ok {
			return nil, fmt.Errorf("auth key not found in secret")
		}

		return map[string]interface{}{
			"type":  "token",
			"token": base64.StdEncoding.EncodeToString(tokenData),
		}, nil
	}

	if auth.KubeconfigSecretRef != nil {
		secret := &corev1.Secret{}
		namespace := auth.KubeconfigSecretRef.Namespace
		if namespace == "" {
			namespace = "default"
		}

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      auth.KubeconfigSecretRef.Name,
			Namespace: namespace,
		}, secret)
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

	if auth.ClientCertificateRef != nil {
		secret := &corev1.Secret{}
		namespace := auth.ClientCertificateRef.Namespace
		if namespace == "" {
			namespace = "default"
		}

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      auth.ClientCertificateRef.Name,
			Namespace: namespace,
		}, secret)
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

	return nil, nil // No auth configured
}

func extractCAFromKubeconfig(kubeconfigB64 string, log *logger.Logger) []byte {
	kubeconfigData, err := base64.StdEncoding.DecodeString(kubeconfigB64)
	if err != nil {
		log.Warn().Err(err).Msg("failed to decode kubeconfig for CA extraction")
		return nil
	}

	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfigData)
	if err != nil {
		log.Warn().Err(err).Msg("failed to parse kubeconfig for CA extraction")
		return nil
	}

	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		log.Warn().Err(err).Msg("failed to get raw kubeconfig for CA extraction")
		return nil
	}

	// Get the current context
	currentContext := rawConfig.CurrentContext
	if currentContext == "" {
		log.Warn().Msg("no current context in kubeconfig for CA extraction")
		return nil
	}

	context, exists := rawConfig.Contexts[currentContext]
	if !exists {
		log.Warn().Str("context", currentContext).Msg("current context not found in kubeconfig for CA extraction")
		return nil
	}

	// Get cluster info
	cluster, exists := rawConfig.Clusters[context.Cluster]
	if !exists {
		log.Warn().Str("cluster", context.Cluster).Msg("cluster not found in kubeconfig for CA extraction")
		return nil
	}

	if len(cluster.CertificateAuthorityData) > 0 {
		return cluster.CertificateAuthorityData
	}

	log.Warn().Msg("no CA data found in kubeconfig")
	return nil
}

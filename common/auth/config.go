package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gatewayv1alpha1 "github.com/openmfp/kubernetes-graphql-gateway/common/apis/v1alpha1"
)

// BuildConfig creates a rest.Config from cluster connection parameters
// This function unifies the authentication logic used by both listener and gateway
func BuildConfig(ctx context.Context, host string, auth *gatewayv1alpha1.AuthConfig, ca *gatewayv1alpha1.CAConfig, k8sClient client.Client) (*rest.Config, error) {
	if host == "" {
		return nil, errors.New("host is required")
	}

	config := &rest.Config{
		Host: host,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true, // Start with insecure, will be overridden if CA is provided
		},
	}

	// Handle CA configuration first
	if ca != nil {
		caData, err := ExtractCAData(ctx, ca, k8sClient)
		if err != nil {
			return nil, errors.Join(errors.New("failed to extract CA data"), err)
		}
		if caData != nil {
			config.TLSClientConfig.CAData = caData
			config.TLSClientConfig.Insecure = false // Use proper TLS verification when CA is provided
		}
	}

	// Handle Auth configuration
	if auth != nil {
		err := ConfigureAuthentication(ctx, config, auth, k8sClient)
		if err != nil {
			return nil, errors.Join(errors.New("failed to configure authentication"), err)
		}
	}

	return config, nil
}

// BuildConfigFromMetadata creates a rest.Config from base64-encoded metadata (used by gateway)
func BuildConfigFromMetadata(host string, authType, token, kubeconfig, certData, keyData, caData string) (*rest.Config, error) {
	if host == "" {
		return nil, errors.New("host is required")
	}

	config := &rest.Config{
		Host: host,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true, // Start with insecure, will be overridden if CA is provided
		},
	}

	// Handle CA data
	if caData != "" {
		decodedCA, err := base64.StdEncoding.DecodeString(caData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode CA data: %w", err)
		}
		config.TLSClientConfig.CAData = decodedCA
		config.TLSClientConfig.Insecure = false
	}

	// Handle authentication based on type
	switch authType {
	case "token":
		if token != "" {
			tokenData, err := base64.StdEncoding.DecodeString(token)
			if err != nil {
				return nil, fmt.Errorf("failed to decode token: %w", err)
			}
			config.BearerToken = string(tokenData)
		}
	case "kubeconfig":
		if kubeconfig != "" {
			kubeconfigData, err := base64.StdEncoding.DecodeString(kubeconfig)
			if err != nil {
				return nil, fmt.Errorf("failed to decode kubeconfig: %w", err)
			}

			if err := ConfigureFromKubeconfig(config, kubeconfigData); err != nil {
				return nil, fmt.Errorf("failed to configure from kubeconfig: %w", err)
			}
		}
	case "clientCert":
		if certData != "" && keyData != "" {
			decodedCert, err := base64.StdEncoding.DecodeString(certData)
			if err != nil {
				return nil, fmt.Errorf("failed to decode cert data: %w", err)
			}
			decodedKey, err := base64.StdEncoding.DecodeString(keyData)
			if err != nil {
				return nil, fmt.Errorf("failed to decode key data: %w", err)
			}
			config.TLSClientConfig.CertData = decodedCert
			config.TLSClientConfig.KeyData = decodedKey
		}
	}

	return config, nil
}

// ExtractCAData extracts CA certificate data from secret or configmap references
func ExtractCAData(ctx context.Context, ca *gatewayv1alpha1.CAConfig, k8sClient client.Client) ([]byte, error) {
	if ca == nil {
		return nil, nil
	}

	if ca.SecretRef != nil {
		secret := &corev1.Secret{}
		namespace := ca.SecretRef.Namespace
		if namespace == "" {
			namespace = "default" // Use default namespace if not specified
		}

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      ca.SecretRef.Name,
			Namespace: namespace,
		}, secret)
		if err != nil {
			return nil, errors.Join(errors.New("failed to get CA secret"), err)
		}

		caData, ok := secret.Data[ca.SecretRef.Key]
		if !ok {
			return nil, errors.New("CA key not found in secret")
		}

		return caData, nil
	}

	if ca.ConfigMapRef != nil {
		configMap := &corev1.ConfigMap{}
		namespace := ca.ConfigMapRef.Namespace
		if namespace == "" {
			namespace = "default"
		}

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      ca.ConfigMapRef.Name,
			Namespace: namespace,
		}, configMap)
		if err != nil {
			return nil, errors.Join(errors.New("failed to get CA config map"), err)
		}

		caData, ok := configMap.Data[ca.ConfigMapRef.Key]
		if !ok {
			return nil, errors.New("CA key not found in config map")
		}

		return []byte(caData), nil
	}

	return nil, nil // No CA configuration
}

// ConfigureAuthentication configures authentication for rest.Config from AuthConfig
func ConfigureAuthentication(ctx context.Context, config *rest.Config, auth *gatewayv1alpha1.AuthConfig, k8sClient client.Client) error {
	if auth == nil {
		return nil
	}

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
			return errors.Join(errors.New("failed to get auth secret"), err)
		}

		tokenData, ok := secret.Data[auth.SecretRef.Key]
		if !ok {
			return errors.New("auth key not found in secret")
		}

		config.BearerToken = string(tokenData)
		return nil
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
			return errors.Join(errors.New("failed to get kubeconfig secret"), err)
		}

		kubeconfigData, ok := secret.Data["kubeconfig"]
		if !ok {
			return errors.New("kubeconfig key not found in secret")
		}

		return ConfigureFromKubeconfig(config, kubeconfigData)
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
			return errors.Join(errors.New("failed to get client certificate secret"), err)
		}

		certData, certOk := secret.Data["tls.crt"]
		keyData, keyOk := secret.Data["tls.key"]

		if !certOk || !keyOk {
			return errors.New("client certificate or key not found in secret")
		}

		config.TLSClientConfig.CertData = certData
		config.TLSClientConfig.KeyData = keyData
		return nil
	}

	if auth.ServiceAccount != nil {
		var expirationSeconds int64
		if auth.ServiceAccount.TokenExpiration != nil {
			// If TokenExpiration is provided, use its value
			expirationSeconds = int64(auth.ServiceAccount.TokenExpiration.Duration.Seconds())
		} else {
			// If TokenExpiration is nil, use the desired default (3600 seconds = 1 hour)
			expirationSeconds = 3600
			fmt.Println("Warning: auth.ServiceAccount.TokenExpiration is nil, defaulting to 3600 seconds.")
		}

		// Build the TokenRequest object
		tokenRequest := &authv1.TokenRequest{
			Spec: authv1.TokenRequestSpec{
				Audiences: auth.ServiceAccount.Audience,
				// Optionally set ExpirationSeconds, BoundObjectRef, etc.
				ExpirationSeconds: &expirationSeconds,
			},
		}

		// Get the service account token using the Kubernetes API
		sa := &corev1.ServiceAccount{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      auth.ServiceAccount.Name,
			Namespace: auth.ServiceAccount.Namespace,
		}, sa)
		if err != nil {
			return errors.Join(errors.New("failed to get service account"), err)
		}

		err = k8sClient.SubResource("token").Create(ctx, sa, tokenRequest)
		if err != nil {
			return errors.Join(errors.New("failed to create token request for service account"), err)
		}

		if tokenRequest.Status.Token == "" {
			return errors.New("received empty token from TokenRequest API")
		}

		config.BearerToken = tokenRequest.Status.Token
		return nil
	}

	// No authentication configured - this might work for some clusters
	return nil
}

// ConfigureFromKubeconfig configures authentication from kubeconfig data
func ConfigureFromKubeconfig(config *rest.Config, kubeconfigData []byte) error {
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

	// Extract authentication information
	return ExtractAuthFromKubeconfig(config, authInfo)
}

// ExtractAuthFromKubeconfig extracts authentication info from kubeconfig AuthInfo
func ExtractAuthFromKubeconfig(config *rest.Config, authInfo *api.AuthInfo) error {
	if authInfo.Token != "" {
		config.BearerToken = authInfo.Token
		return nil
	}

	if authInfo.TokenFile != "" {
		// TODO: Read token from file if needed
		return errors.New("token file authentication not yet implemented")
	}

	if len(authInfo.ClientCertificateData) > 0 && len(authInfo.ClientKeyData) > 0 {
		config.TLSClientConfig.CertData = authInfo.ClientCertificateData
		config.TLSClientConfig.KeyData = authInfo.ClientKeyData
		return nil
	}

	if authInfo.ClientCertificate != "" && authInfo.ClientKey != "" {
		config.TLSClientConfig.CertFile = authInfo.ClientCertificate
		config.TLSClientConfig.KeyFile = authInfo.ClientKey
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

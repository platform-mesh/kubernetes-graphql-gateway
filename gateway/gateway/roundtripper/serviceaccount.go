package roundtripper

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/transport"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceAccountConfig contains configuration for service account token generation
type ServiceAccountConfig struct {
	Name                   string
	Namespace              string
	Audience               []string
	TokenExpirationSeconds int64
}

// serviceAccountRoundTripper generates service account tokens dynamically
type serviceAccountRoundTripper struct {
	delegate  http.RoundTripper
	k8sClient client.Client
	saConfig  ServiceAccountConfig

	// Token caching to avoid excessive TokenRequest API calls
	mu             sync.RWMutex
	cachedToken    string
	tokenExpiresAt time.Time
}

// NewServiceAccountRoundTripper creates a RoundTripper that authenticates using dynamically generated service account tokens
func NewServiceAccountRoundTripper(
	delegate http.RoundTripper,
	k8sClient client.Client,
	saConfig ServiceAccountConfig,
) http.RoundTripper {
	return &serviceAccountRoundTripper{
		delegate:  delegate,
		k8sClient: k8sClient,
		saConfig:  saConfig,
	}
}

func (rt *serviceAccountRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := rt.getOrRefreshToken(req.Context())
	if err != nil {
		return nil, fmt.Errorf("failed to get service account token: %w", err)
	}

	// Use bearer auth with the generated token
	return transport.NewBearerAuthRoundTripper(token, rt.delegate).RoundTrip(req)
}

// getOrRefreshToken returns a cached token or generates a new one if expired
func (rt *serviceAccountRoundTripper) getOrRefreshToken(ctx context.Context) (string, error) {
	rt.mu.RLock()
	if rt.cachedToken != "" && time.Now().Before(rt.tokenExpiresAt) {
		token := rt.cachedToken
		rt.mu.RUnlock()
		return token, nil
	}
	rt.mu.RUnlock()

	// Need to refresh token
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// Double-check after acquiring write lock
	if rt.cachedToken != "" && time.Now().Before(rt.tokenExpiresAt) {
		return rt.cachedToken, nil
	}

	token, expiresAt, err := rt.generateToken(ctx)
	if err != nil {
		return "", err
	}

	rt.cachedToken = token
	rt.tokenExpiresAt = expiresAt

	return token, nil
}

// generateToken creates a new token using the TokenRequest API
func (rt *serviceAccountRoundTripper) generateToken(ctx context.Context) (string, time.Time, error) {
	expirationSeconds := rt.saConfig.TokenExpirationSeconds
	if expirationSeconds <= 0 {
		expirationSeconds = 3600 // Default to 1 hour
	}

	tokenRequest := &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			Audiences:         rt.saConfig.Audience,
			ExpirationSeconds: &expirationSeconds,
		},
	}

	sa := &corev1.ServiceAccount{}
	err := rt.k8sClient.Get(ctx, types.NamespacedName{
		Name:      rt.saConfig.Name,
		Namespace: rt.saConfig.Namespace,
	}, sa)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to get service account %s/%s: %w",
			rt.saConfig.Namespace, rt.saConfig.Name, err)
	}

	err = rt.k8sClient.SubResource("token").Create(ctx, sa, tokenRequest)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create token for service account %s/%s: %w",
			rt.saConfig.Namespace, rt.saConfig.Name, err)
	}

	if tokenRequest.Status.Token == "" {
		return "", time.Time{}, fmt.Errorf("received empty token from TokenRequest API")
	}

	// Use the API server's actual expiration timestamp (it may differ from requested)
	// Fall back to calculated expiration if ExpirationTimestamp is not set
	var expiresAt time.Time
	if !tokenRequest.Status.ExpirationTimestamp.IsZero() {
		actualLifetime := time.Until(tokenRequest.Status.ExpirationTimestamp.Time)
		// Refresh 30 seconds before expiration or 10% of token lifetime, whichever is smaller
		buffer := min(actualLifetime/10, 30*time.Second)
		expiresAt = tokenRequest.Status.ExpirationTimestamp.Add(-buffer)
	} else {
		// Fallback: use requested expiration if API didn't return timestamp
		buffer := min(time.Duration(expirationSeconds/10)*time.Second, 30*time.Second)
		expiresAt = time.Now().Add(time.Duration(expirationSeconds)*time.Second - buffer)
	}

	return tokenRequest.Status.Token, expiresAt, nil
}

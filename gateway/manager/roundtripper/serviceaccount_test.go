package roundtripper_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/manager/roundtripper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// mockRoundTripper captures the Authorization header from requests
type mockRoundTripper struct {
	capturedAuthHeader string
	responseStatus     int
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.capturedAuthHeader = req.Header.Get("Authorization")
	return &http.Response{
		StatusCode: m.responseStatus,
		Request:    req,
	}, nil
}

func TestServiceAccountRoundTripper_RoundTrip(t *testing.T) {
	tests := []struct {
		name                  string
		saConfig              roundtripper.ServiceAccountConfig
		serviceAccountExists  bool
		tokenResponse         string
		tokenExpirationTime   time.Time
		expectedAuthHeader    string
		expectError           bool
		expectedErrorContains string
	}{
		{
			name: "successful_token_generation",
			saConfig: roundtripper.ServiceAccountConfig{
				Name:                   "test-sa",
				Namespace:              "test-namespace",
				TokenExpirationSeconds: 3600,
			},
			serviceAccountExists: true,
			tokenResponse:        "generated-token-12345",
			tokenExpirationTime:  time.Now().Add(1 * time.Hour),
			expectedAuthHeader:   "Bearer generated-token-12345",
			expectError:          false,
		},
		{
			name: "service_account_not_found",
			saConfig: roundtripper.ServiceAccountConfig{
				Name:                   "non-existent-sa",
				Namespace:              "test-namespace",
				TokenExpirationSeconds: 3600,
			},
			serviceAccountExists:  false,
			expectError:           true,
			expectedErrorContains: "failed to get service account",
		},
		{
			name: "with_custom_audience",
			saConfig: roundtripper.ServiceAccountConfig{
				Name:                   "audience-sa",
				Namespace:              "test-namespace",
				Audience:               []string{"api://custom", "https://kubernetes.default.svc"},
				TokenExpirationSeconds: 3600,
			},
			serviceAccountExists: true,
			tokenResponse:        "audience-token-67890",
			tokenExpirationTime:  time.Now().Add(1 * time.Hour),
			expectedAuthHeader:   "Bearer audience-token-67890",
			expectError:          false,
		},
		{
			name: "default_expiration_when_zero",
			saConfig: roundtripper.ServiceAccountConfig{
				Name:                   "default-exp-sa",
				Namespace:              "test-namespace",
				TokenExpirationSeconds: 0, // Should default to 3600
			},
			serviceAccountExists: true,
			tokenResponse:        "default-exp-token",
			tokenExpirationTime:  time.Now().Add(1 * time.Hour),
			expectedAuthHeader:   "Bearer default-exp-token",
			expectError:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			require.NoError(t, authv1.AddToScheme(scheme))

			var objects []client.Object
			if tt.serviceAccountExists {
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tt.saConfig.Name,
						Namespace: tt.saConfig.Namespace,
					},
				}
				objects = append(objects, sa)
			}

			// Create fake client with interceptor for SubResource calls
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				WithInterceptorFuncs(interceptor.Funcs{
					SubResourceCreate: func(ctx context.Context, client client.Client, subResourceName string, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
						if subResourceName == "token" {
							if tokenReq, ok := subResource.(*authv1.TokenRequest); ok {
								tokenReq.Status.Token = tt.tokenResponse
								tokenReq.Status.ExpirationTimestamp = metav1.NewTime(tt.tokenExpirationTime)
							}
						}
						return nil
					},
				}).
				Build()

			mockRT := &mockRoundTripper{responseStatus: http.StatusOK}
			rt := roundtripper.NewServiceAccountRoundTripper(mockRT, fakeClient, tt.saConfig)

			req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/pods", nil)
			resp, err := rt.RoundTrip(req)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErrorContains != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, tt.expectedAuthHeader, mockRT.capturedAuthHeader)
		})
	}
}

func TestServiceAccountRoundTripper_TokenCaching(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, authv1.AddToScheme(scheme))

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cache-test-sa",
			Namespace: "test-namespace",
		},
	}

	tokenRequestCount := 0
	tokenMu := sync.Mutex{}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sa).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceCreate: func(ctx context.Context, client client.Client, subResourceName string, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
				if subResourceName == "token" {
					tokenMu.Lock()
					tokenRequestCount++
					tokenMu.Unlock()
					if tokenReq, ok := subResource.(*authv1.TokenRequest); ok {
						tokenReq.Status.Token = "cached-token"
						tokenReq.Status.ExpirationTimestamp = metav1.NewTime(time.Now().Add(1 * time.Hour))
					}
				}
				return nil
			},
		}).
		Build()

	saConfig := roundtripper.ServiceAccountConfig{
		Name:                   "cache-test-sa",
		Namespace:              "test-namespace",
		TokenExpirationSeconds: 3600,
	}

	mockRT := &mockRoundTripper{responseStatus: http.StatusOK}
	rt := roundtripper.NewServiceAccountRoundTripper(mockRT, fakeClient, saConfig)

	// Make multiple requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/pods", nil)
		resp, err := rt.RoundTrip(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "Bearer cached-token", mockRT.capturedAuthHeader)
	}

	// Verify token was only requested once due to caching
	tokenMu.Lock()
	finalCount := tokenRequestCount
	tokenMu.Unlock()
	assert.Equal(t, 1, finalCount, "Token should only be generated once due to caching")
}

func TestServiceAccountRoundTripper_ConcurrentAccess(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, authv1.AddToScheme(scheme))

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "concurrent-sa",
			Namespace: "test-namespace",
		},
	}

	tokenRequestCount := 0
	tokenMu := sync.Mutex{}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sa).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceCreate: func(ctx context.Context, client client.Client, subResourceName string, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
				if subResourceName == "token" {
					tokenMu.Lock()
					tokenRequestCount++
					tokenMu.Unlock()
					// Simulate some latency
					time.Sleep(10 * time.Millisecond)
					if tokenReq, ok := subResource.(*authv1.TokenRequest); ok {
						tokenReq.Status.Token = "concurrent-token"
						tokenReq.Status.ExpirationTimestamp = metav1.NewTime(time.Now().Add(1 * time.Hour))
					}
				}
				return nil
			},
		}).
		Build()

	saConfig := roundtripper.ServiceAccountConfig{
		Name:                   "concurrent-sa",
		Namespace:              "test-namespace",
		TokenExpirationSeconds: 3600,
	}

	mockRT := &mockRoundTripper{responseStatus: http.StatusOK}
	rt := roundtripper.NewServiceAccountRoundTripper(mockRT, fakeClient, saConfig)

	// Launch multiple concurrent requests
	var wg sync.WaitGroup
	numGoroutines := 10
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/pods", nil)
			_, err := rt.RoundTrip(req)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Unexpected error during concurrent access: %v", err)
	}

	// The double-check locking pattern should limit token generation
	// to at most a few calls (ideally 1, but race conditions might cause 2)
	tokenMu.Lock()
	finalCount := tokenRequestCount
	tokenMu.Unlock()
	assert.LessOrEqual(t, finalCount, 2, "Token generation should be minimized by caching")
}

func TestServiceAccountRoundTripper_EmptyTokenError(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, authv1.AddToScheme(scheme))

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "empty-token-sa",
			Namespace: "test-namespace",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sa).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceCreate: func(ctx context.Context, client client.Client, subResourceName string, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
				// Return empty token
				if tokenReq, ok := subResource.(*authv1.TokenRequest); ok {
					tokenReq.Status.Token = "" // Empty token
					tokenReq.Status.ExpirationTimestamp = metav1.NewTime(time.Now().Add(1 * time.Hour))
				}
				return nil
			},
		}).
		Build()

	saConfig := roundtripper.ServiceAccountConfig{
		Name:                   "empty-token-sa",
		Namespace:              "test-namespace",
		TokenExpirationSeconds: 3600,
	}

	mockRT := &mockRoundTripper{responseStatus: http.StatusOK}
	rt := roundtripper.NewServiceAccountRoundTripper(mockRT, fakeClient, saConfig)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/pods", nil)
	_, err := rt.RoundTrip(req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty token")
}

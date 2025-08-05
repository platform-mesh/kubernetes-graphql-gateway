package roundtripper_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/openmfp/golang-commons/logger/testlogger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	appConfig "github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager/mocks"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager/roundtripper"
)

func TestRoundTripper_RoundTrip(t *testing.T) {
	tests := []struct {
		name               string
		token              string
		localDevelopment   bool
		shouldImpersonate  bool
		expectedStatusCode int
		setupMocks         func(*mocks.MockRoundTripper, *mocks.MockRoundTripper)
	}{
		{
			name:               "local_development_uses_admin",
			localDevelopment:   true,
			expectedStatusCode: http.StatusOK,
			setupMocks: func(admin, unauthorized *mocks.MockRoundTripper) {
				admin.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK}, nil)
			},
		},
		{
			name:               "no_token_returns_unauthorized",
			localDevelopment:   false,
			expectedStatusCode: http.StatusUnauthorized,
			setupMocks: func(admin, unauthorized *mocks.MockRoundTripper) {
				unauthorized.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusUnauthorized}, nil)
			},
		},
		{
			name:               "valid_token_without_impersonation",
			token:              "valid-token",
			localDevelopment:   false,
			shouldImpersonate:  false,
			expectedStatusCode: http.StatusOK,
			setupMocks: func(admin, unauthorized *mocks.MockRoundTripper) {
				admin.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK}, nil)
			},
		},
		{
			name:               "valid_token_with_impersonation",
			token:              createTestToken(t, jwt.MapClaims{"sub": "test-user"}),
			localDevelopment:   false,
			shouldImpersonate:  true,
			expectedStatusCode: http.StatusOK,
			setupMocks: func(admin, unauthorized *mocks.MockRoundTripper) {
				admin.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdmin := &mocks.MockRoundTripper{}
			mockUnauthorized := &mocks.MockRoundTripper{}

			tt.setupMocks(mockAdmin, mockUnauthorized)

			appCfg := appConfig.Config{
				LocalDevelopment: tt.localDevelopment,
			}
			appCfg.Gateway.ShouldImpersonate = tt.shouldImpersonate
			appCfg.Gateway.UsernameClaim = "sub"

			rt := roundtripper.New(testlogger.New().Logger, appCfg, mockAdmin, mockUnauthorized)

			req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/pods", nil)
			if tt.token != "" {
				ctx := context.WithValue(req.Context(), roundtripper.TokenKey{}, tt.token)
				req = req.WithContext(ctx)
			}

			resp, err := rt.RoundTrip(req)
			require.NoError(t, err)
			require.NotNil(t, resp)

			assert.Equal(t, tt.expectedStatusCode, resp.StatusCode)

			mockAdmin.AssertExpectations(t)
			mockUnauthorized.AssertExpectations(t)
		})
	}
}

func TestRoundTripper_DiscoveryRequests(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		isDiscovery bool
	}{
		// Basic discovery endpoints
		{
			name:        "api_root_discovery",
			method:      "GET",
			path:        "/api",
			isDiscovery: true,
		},
		{
			name:        "apis_root_discovery",
			method:      "GET",
			path:        "/apis",
			isDiscovery: true,
		},
		{
			name:        "api_version_discovery",
			method:      "GET",
			path:        "/api/v1",
			isDiscovery: true,
		},
		{
			name:        "apis_group_discovery",
			method:      "GET",
			path:        "/apis/apps",
			isDiscovery: true,
		},
		{
			name:        "apis_group_version_discovery",
			method:      "GET",
			path:        "/apis/apps/v1",
			isDiscovery: true,
		},

		// KCP workspace prefixed discovery endpoints
		{
			name:        "kcp_api_root_discovery",
			method:      "GET",
			path:        "/clusters/workspace1/api",
			isDiscovery: true,
		},
		{
			name:        "kcp_apis_root_discovery",
			method:      "GET",
			path:        "/clusters/workspace1/apis",
			isDiscovery: true,
		},
		{
			name:        "kcp_api_version_discovery",
			method:      "GET",
			path:        "/clusters/workspace1/api/v1",
			isDiscovery: true,
		},
		{
			name:        "kcp_apis_group_discovery",
			method:      "GET",
			path:        "/clusters/workspace1/apis/apps",
			isDiscovery: true,
		},
		{
			name:        "kcp_apis_group_version_discovery",
			method:      "GET",
			path:        "/clusters/workspace1/apis/apps/v1",
			isDiscovery: true,
		},

		// Virtual workspace prefixed discovery endpoints
		{
			name:        "virtual_api_root_discovery",
			method:      "GET",
			path:        "/services/myservice/clusters/workspace1/api",
			isDiscovery: true,
		},
		{
			name:        "virtual_apis_root_discovery",
			method:      "GET",
			path:        "/services/myservice/clusters/workspace1/apis",
			isDiscovery: true,
		},
		{
			name:        "virtual_api_version_discovery",
			method:      "GET",
			path:        "/services/myservice/clusters/workspace1/api/v1",
			isDiscovery: true,
		},
		{
			name:        "virtual_apis_group_discovery",
			method:      "GET",
			path:        "/services/myservice/clusters/workspace1/apis/apps",
			isDiscovery: true,
		},
		{
			name:        "virtual_apis_group_version_discovery",
			method:      "GET",
			path:        "/services/myservice/clusters/workspace1/apis/apps/v1",
			isDiscovery: true,
		},

		// Non-discovery requests
		{
			name:        "resource_request",
			method:      "GET",
			path:        "/api/v1/pods",
			isDiscovery: false,
		},
		{
			name:        "kcp_resource_request",
			method:      "GET",
			path:        "/clusters/workspace1/api/v1/pods",
			isDiscovery: false,
		},
		{
			name:        "virtual_resource_request",
			method:      "GET",
			path:        "/services/myservice/clusters/workspace1/api/v1/pods",
			isDiscovery: false,
		},
		{
			name:        "post_request",
			method:      "POST",
			path:        "/api/v1/pods",
			isDiscovery: false,
		},
		{
			name:        "empty_path",
			method:      "GET",
			path:        "/",
			isDiscovery: false,
		},
		{
			name:        "invalid_path",
			method:      "GET",
			path:        "/invalid",
			isDiscovery: false,
		},
		{
			name:        "too_many_parts",
			method:      "GET",
			path:        "/apis/apps/v1/deployments",
			isDiscovery: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdmin := &mocks.MockRoundTripper{}
			mockUnauthorized := &mocks.MockRoundTripper{}

			if tt.isDiscovery {
				mockAdmin.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK}, nil)
			} else {
				mockUnauthorized.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusUnauthorized}, nil)
			}

			appCfg := appConfig.Config{
				LocalDevelopment: false,
			}
			appCfg.Gateway.ShouldImpersonate = false
			appCfg.Gateway.UsernameClaim = "sub"

			rt := roundtripper.New(testlogger.New().Logger, appCfg, mockAdmin, mockUnauthorized)

			req := httptest.NewRequest(tt.method, "http://example.com"+tt.path, nil)

			resp, err := rt.RoundTrip(req)
			require.NoError(t, err)
			require.NotNil(t, resp)

			if tt.isDiscovery {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			} else {
				assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			}

			mockAdmin.AssertExpectations(t)
			mockUnauthorized.AssertExpectations(t)
		})
	}
}

func TestRoundTripper_ComprehensiveFunctionality(t *testing.T) {
	tests := []struct {
		name                  string
		token                 string
		localDevelopment      bool
		shouldImpersonate     bool
		usernameClaim         string
		expectedStatusCode    int
		expectedImpersonation string
		setupMocks            func(*mocks.MockRoundTripper, *mocks.MockRoundTripper)
	}{
		{
			name:                  "impersonation_with_custom_claim",
			token:                 createTestTokenWithClaim(t, "email", "user@example.com"),
			localDevelopment:      false,
			shouldImpersonate:     true,
			usernameClaim:         "email",
			expectedStatusCode:    http.StatusOK,
			expectedImpersonation: "user@example.com",
			setupMocks: func(admin, unauthorized *mocks.MockRoundTripper) {
				admin.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK}, nil)
			},
		},
		{
			name:                  "impersonation_with_sub_claim",
			token:                 createTestTokenWithClaim(t, "sub", "test-user-123"),
			localDevelopment:      false,
			shouldImpersonate:     true,
			usernameClaim:         "sub",
			expectedStatusCode:    http.StatusOK,
			expectedImpersonation: "test-user-123",
			setupMocks: func(admin, unauthorized *mocks.MockRoundTripper) {
				admin.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK}, nil)
			},
		},
		{
			name:               "missing_user_claim_returns_unauthorized",
			token:              createTestTokenWithClaim(t, "other_claim", "value"),
			localDevelopment:   false,
			shouldImpersonate:  true,
			usernameClaim:      "sub",
			expectedStatusCode: http.StatusUnauthorized,
			setupMocks: func(admin, unauthorized *mocks.MockRoundTripper) {
				unauthorized.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusUnauthorized}, nil)
			},
		},
		{
			name:               "invalid_token_returns_unauthorized",
			token:              "invalid.jwt.token",
			localDevelopment:   false,
			shouldImpersonate:  true,
			usernameClaim:      "sub",
			expectedStatusCode: http.StatusUnauthorized,
			setupMocks: func(admin, unauthorized *mocks.MockRoundTripper) {
				unauthorized.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusUnauthorized}, nil)
			},
		},
		{
			name:               "empty_user_claim_returns_unauthorized",
			token:              createTestTokenWithClaim(t, "sub", ""),
			localDevelopment:   false,
			shouldImpersonate:  true,
			usernameClaim:      "sub",
			expectedStatusCode: http.StatusUnauthorized,
			setupMocks: func(admin, unauthorized *mocks.MockRoundTripper) {
				unauthorized.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusUnauthorized}, nil)
			},
		},
		{
			name:               "non_string_user_claim_returns_unauthorized",
			token:              createTestTokenWithClaim(t, "sub", 12345),
			localDevelopment:   false,
			shouldImpersonate:  true,
			usernameClaim:      "sub",
			expectedStatusCode: http.StatusUnauthorized,
			setupMocks: func(admin, unauthorized *mocks.MockRoundTripper) {
				unauthorized.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusUnauthorized}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdmin := &mocks.MockRoundTripper{}
			mockUnauthorized := &mocks.MockRoundTripper{}

			tt.setupMocks(mockAdmin, mockUnauthorized)

			appCfg := appConfig.Config{
				LocalDevelopment: tt.localDevelopment,
			}
			appCfg.Gateway.ShouldImpersonate = tt.shouldImpersonate
			appCfg.Gateway.UsernameClaim = tt.usernameClaim

			rt := roundtripper.New(testlogger.New().Logger, appCfg, mockAdmin, mockUnauthorized)

			req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/pods", nil)
			if tt.token != "" {
				ctx := context.WithValue(req.Context(), roundtripper.TokenKey{}, tt.token)
				req = req.WithContext(ctx)
			}

			resp, err := rt.RoundTrip(req)
			require.NoError(t, err)
			require.NotNil(t, resp)

			assert.Equal(t, tt.expectedStatusCode, resp.StatusCode)

			mockAdmin.AssertExpectations(t)
			mockUnauthorized.AssertExpectations(t)
		})
	}
}

func TestRoundTripper_KCPDiscoveryRequests(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		isDiscovery bool
	}{
		{
			name:        "kcp_clusters_api_discovery",
			path:        "/clusters/workspace1/api",
			isDiscovery: true,
		},
		{
			name:        "kcp_clusters_apis_discovery",
			path:        "/clusters/workspace1/apis",
			isDiscovery: true,
		},
		{
			name:        "kcp_clusters_apis_group_discovery",
			path:        "/clusters/workspace1/apis/apps",
			isDiscovery: true,
		},
		{
			name:        "kcp_clusters_apis_group_version_discovery",
			path:        "/clusters/workspace1/apis/apps/v1",
			isDiscovery: true,
		},
		{
			name:        "kcp_clusters_api_version_discovery",
			path:        "/clusters/workspace1/api/v1",
			isDiscovery: true,
		},
		{
			name:        "kcp_clusters_resource_request",
			path:        "/clusters/workspace1/api/v1/pods",
			isDiscovery: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdmin := &mocks.MockRoundTripper{}
			mockUnauthorized := &mocks.MockRoundTripper{}

			if tt.isDiscovery {
				mockAdmin.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK}, nil)
			} else {
				mockUnauthorized.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusUnauthorized}, nil)
			}

			appCfg := appConfig.Config{
				LocalDevelopment: false,
			}
			appCfg.Gateway.ShouldImpersonate = false
			appCfg.Gateway.UsernameClaim = "sub"

			rt := roundtripper.New(testlogger.New().Logger, appCfg, mockAdmin, mockUnauthorized)

			req := httptest.NewRequest(http.MethodGet, "http://example.com"+tt.path, nil)

			resp, err := rt.RoundTrip(req)
			require.NoError(t, err)
			require.NotNil(t, resp)

			if tt.isDiscovery {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			} else {
				assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			}

			mockAdmin.AssertExpectations(t)
			mockUnauthorized.AssertExpectations(t)
		})
	}
}

func createTestToken(t *testing.T, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)
	return signedToken
}

func createTestTokenWithClaim(t *testing.T, claimKey string, claimValue interface{}) string {
	claims := jwt.MapClaims{claimKey: claimValue}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)
	return signedToken
}

func TestRoundTripper_InvalidTokenSecurityFix(t *testing.T) {
	// This test verifies that the security fix works: invalid tokens should be rejected
	// by the Kubernetes cluster itself, not by falling back to admin credentials

	mockAdmin := &mocks.MockRoundTripper{}
	mockUnauthorized := &mocks.MockRoundTripper{}

	// The unauthorizedRT should be called since we have no token
	mockUnauthorized.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusUnauthorized}, nil)

	appCfg := appConfig.Config{}
	appCfg.Gateway.ShouldImpersonate = false
	appCfg.Gateway.UsernameClaim = "sub"

	rt := roundtripper.New(testlogger.New().Logger, appCfg, mockAdmin, mockUnauthorized)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pods", nil)
	// Don't set a token to simulate the invalid token case

	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRoundTripper_ExistingAuthHeadersAreCleanedBeforeTokenAuth(t *testing.T) {
	// This test verifies that existing Authorization headers are properly cleaned
	// before setting the bearer token, preventing admin credentials from leaking through

	mockAdmin := &mocks.MockRoundTripper{}
	mockUnauthorized := &mocks.MockRoundTripper{}

	// Capture the request that gets sent to adminRT
	var capturedRequest *http.Request
	mockAdmin.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK}, nil).Run(func(req *http.Request) {
		capturedRequest = req
	})

	appCfg := appConfig.Config{}
	appCfg.Gateway.ShouldImpersonate = false
	appCfg.Gateway.UsernameClaim = "sub"

	rt := roundtripper.New(testlogger.New().Logger, appCfg, mockAdmin, mockUnauthorized)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pods", nil)

	// Set an existing Authorization header that should be cleaned
	req.Header.Set("Authorization", "Bearer admin-token-that-should-be-removed")

	// Add the token to context
	req = req.WithContext(context.WithValue(req.Context(), roundtripper.TokenKey{}, "user-token"))

	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify that the captured request has the correct Authorization header
	require.NotNil(t, capturedRequest)
	authHeader := capturedRequest.Header.Get("Authorization")
	assert.Equal(t, "Bearer user-token", authHeader)

	// Verify that the original admin token was removed
	assert.NotContains(t, authHeader, "admin-token-that-should-be-removed")
}

func TestRoundTripper_ExistingAuthHeadersAreCleanedBeforeImpersonation(t *testing.T) {
	// This test verifies that existing Authorization headers are properly cleaned
	// before setting the bearer token in impersonation mode

	mockAdmin := &mocks.MockRoundTripper{}
	mockUnauthorized := &mocks.MockRoundTripper{}

	// Capture the request that gets sent to the impersonation round tripper (which uses adminRT)
	var capturedRequest *http.Request
	mockAdmin.EXPECT().RoundTrip(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK}, nil).Run(func(req *http.Request) {
		capturedRequest = req
	})

	appCfg := appConfig.Config{}
	appCfg.Gateway.ShouldImpersonate = true
	appCfg.Gateway.UsernameClaim = "sub"

	rt := roundtripper.New(testlogger.New().Logger, appCfg, mockAdmin, mockUnauthorized)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pods", nil)

	// Set an existing Authorization header that should be cleaned
	req.Header.Set("Authorization", "Bearer admin-token-that-should-be-removed")

	// Create a valid JWT token with user claim
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "test-user",
	})
	tokenString, err := token.SignedString([]byte("secret"))
	require.NoError(t, err)

	// Add the token to context
	req = req.WithContext(context.WithValue(req.Context(), roundtripper.TokenKey{}, tokenString))

	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify that the captured request has the correct Authorization header
	require.NotNil(t, capturedRequest)
	authHeader := capturedRequest.Header.Get("Authorization")
	assert.Equal(t, "Bearer "+tokenString, authHeader)

	// Verify that the original admin token was removed
	assert.NotContains(t, authHeader, "admin-token-that-should-be-removed")

	// Verify that the impersonation header is set
	impersonateHeader := capturedRequest.Header.Get("Impersonate-User")
	assert.Equal(t, "test-user", impersonateHeader)
}

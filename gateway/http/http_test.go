package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	utilscontext "github.com/platform-mesh/kubernetes-graphql-gateway/gateway/utils/context"
)

// captureHandler is a test handler that records the request context values
// passed through the middleware chain.
type captureHandler struct {
	called      bool
	token       string
	tokenOK     bool
	clusterName string
	clusterOK   bool
}

func (h *captureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.called = true
	h.token, h.tokenOK = utilscontext.GetTokenFromCtx(r.Context())
	h.clusterName, h.clusterOK = utilscontext.GetClusterFromCtx(r.Context())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// newTestServer creates a Server wrapping the given gateway handler for testing.
func newTestServer(t *testing.T, gateway http.Handler) *httptest.Server {
	t.Helper()
	srv, err := NewServer(ServerConfig{
		Gateway:    gateway,
		Addr:       ":0",
		CORSConfig: CORSConfig{},
	})
	require.NoError(t, err)
	return httptest.NewServer(srv.Server.Handler)
}

func TestMissingAuthorizationHeader(t *testing.T) {
	handler := &captureHandler{}
	ts := newTestServer(t, handler)
	defer ts.Close()

	req, err := http.NewRequest("POST", ts.URL+"/api/clusters/test-cluster", strings.NewReader(`{"query":"{}"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.False(t, handler.called, "gateway handler should not be called when auth header is missing")

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "missing Authorization header")
}

func TestInvalidAuthorizationFormat(t *testing.T) {
	handler := &captureHandler{}
	ts := newTestServer(t, handler)
	defer ts.Close()

	req, err := http.NewRequest("POST", ts.URL+"/api/clusters/test-cluster", strings.NewReader(`{"query":"{}"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic abc123")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.False(t, handler.called, "gateway handler should not be called for non-Bearer auth")

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "invalid Authorization header format")
}

func TestEmptyBearerToken(t *testing.T) {
	handler := &captureHandler{}
	ts := newTestServer(t, handler)
	defer ts.Close()

	req, err := http.NewRequest("POST", ts.URL+"/api/clusters/test-cluster", strings.NewReader(`{"query":"{}"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	// Go's HTTP client trims trailing whitespace from header values, so
	// "Bearer " becomes "Bearer" on the wire. This fails the HasPrefix
	// check for "Bearer " (with space), resulting in the "invalid format"
	// error rather than "empty bearer token". Either way the request is
	// rejected with 401, which is the behavior under test.
	req.Header.Set("Authorization", "Bearer ")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.False(t, handler.called, "gateway handler should not be called for empty bearer token")
}

func TestValidBearerTokenForwarded(t *testing.T) {
	handler := &captureHandler{}
	ts := newTestServer(t, handler)
	defer ts.Close()

	req, err := http.NewRequest("POST", ts.URL+"/api/clusters/my-cluster", strings.NewReader(`{"query":"{}"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-test-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, handler.called, "gateway handler should be called with valid bearer token")
	assert.True(t, handler.tokenOK, "token should be present in context")
	assert.Equal(t, "valid-test-token", handler.token)
	assert.True(t, handler.clusterOK, "cluster name should be present in context")
	assert.Equal(t, "my-cluster", handler.clusterName)
}

func TestHealthEndpointNoAuth(t *testing.T) {
	handler := &captureHandler{}
	ts := newTestServer(t, handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.False(t, handler.called, "gateway handler should not be called for /healthz")
}

func TestReadinessEndpointNoAuth(t *testing.T) {
	handler := &captureHandler{}
	ts := newTestServer(t, handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/readyz")
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.False(t, handler.called, "gateway handler should not be called for /readyz")
}

func TestMetricsEndpointNoAuth(t *testing.T) {
	handler := &captureHandler{}
	ts := newTestServer(t, handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.False(t, handler.called, "gateway handler should not be called for /metrics")
}

func TestBearerTokenWithExtraSpaces(t *testing.T) {
	// "Bearer  token-with-leading-space" — the token after TrimPrefix("Bearer ") is " token-with-leading-space"
	// This is a valid (non-empty) token string, so it should be forwarded
	handler := &captureHandler{}
	ts := newTestServer(t, handler)
	defer ts.Close()

	req, err := http.NewRequest("POST", ts.URL+"/api/clusters/test-cluster", strings.NewReader(`{"query":"{}"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer  extra-space-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	// The token will be " extra-space-token" (with leading space), which is non-empty
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, handler.called)
	assert.Equal(t, " extra-space-token", handler.token, "raw token after 'Bearer ' prefix should be preserved")
}

func TestClusterNameExtraction(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		expectedCluster string
		expectCalled    bool
	}{
		{
			name:            "simple cluster name",
			path:            "/api/clusters/my-cluster",
			expectedCluster: "my-cluster",
			expectCalled:    true,
		},
		{
			name:            "cluster name with dots",
			path:            "/api/clusters/cluster.example.com",
			expectedCluster: "cluster.example.com",
			expectCalled:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := &captureHandler{}
			ts := newTestServer(t, handler)
			defer ts.Close()

			req, err := http.NewRequest("POST", ts.URL+tc.path, strings.NewReader(`{"query":"{}"}`))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer some-token")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close() //nolint:errcheck

			assert.Equal(t, tc.expectCalled, handler.called)
			if tc.expectCalled {
				assert.Equal(t, tc.expectedCluster, handler.clusterName)
			}
		})
	}
}

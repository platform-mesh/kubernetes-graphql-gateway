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

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.False(t, handler.called)

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
	assert.False(t, handler.called)
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
	assert.True(t, handler.called)
	assert.Equal(t, "valid-test-token", handler.token)
	assert.Equal(t, "my-cluster", handler.clusterName)
}

func TestUnauthenticatedEndpoints(t *testing.T) {
	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
		t.Run(path, func(t *testing.T) {
			handler := &captureHandler{}
			ts := newTestServer(t, handler)
			defer ts.Close()

			resp, err := http.Get(ts.URL + path)
			require.NoError(t, err)
			defer resp.Body.Close() //nolint:errcheck

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.False(t, handler.called)
		})
	}
}

package manager_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager"
	"sigs.k8s.io/controller-runtime/pkg/kontext"
)

func TestServeHTTP_CORSPreflight(t *testing.T) {
	s := manager.NewManagerForTest()
	req := httptest.NewRequest(http.MethodOptions, "/testws/graphql", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for CORS preflight, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("CORS headers not set")
	}
}

func TestServeHTTP_InvalidWorkspace(t *testing.T) {
	s := manager.NewManagerForTest()
	req := httptest.NewRequest(http.MethodGet, "/invalidws/graphql", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for invalid workspace, got %d", w.Code)
	}
}

func TestServeHTTP_AuthRequired_NoToken(t *testing.T) {
	s := manager.NewManagerForTest()
	s.AppCfg.LocalDevelopment = false
	req := httptest.NewRequest(http.MethodPost, "/testws/graphql", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing token, got %d", w.Code)
	}
}

func TestServeHTTP_CheckClusterNameInRequest(t *testing.T) {
	s := manager.NewManagerForTest()
	s.AppCfg.EnableKcp = true
	s.AppCfg.LocalDevelopment = true

	var capturedCtx context.Context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})
	s.SetHandlerForTest("testws", testHandler)

	req := httptest.NewRequest(http.MethodPost, "/testws/graphql", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	cluster, ok := kontext.ClusterFrom(capturedCtx)
	if !ok || cluster != logicalcluster.Name("testws") {
		t.Errorf("expected workspace 'testws' in context, got %v (found: %t)", cluster, ok)
	}

	token, ok := capturedCtx.Value(manager.TokenKey{}).(string)
	if !ok || token != "test-token" {
		t.Errorf("expected token 'test-token' in context, got %v (found: %t)", token, ok)
	}
}

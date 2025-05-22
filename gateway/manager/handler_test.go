package manager_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager"
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

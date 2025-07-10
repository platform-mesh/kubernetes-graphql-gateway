package targetcluster_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/kcp-dev/logicalcluster/v3"
	"sigs.k8s.io/controller-runtime/pkg/kontext"

	"github.com/openmfp/golang-commons/logger"
	appConfig "github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager/roundtripper"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager/targetcluster"
)

func TestGetToken(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		expectedToken string
	}{
		{
			name:          "Bearer token",
			authorization: "Bearer abc123",
			expectedToken: "abc123",
		},
		{
			name:          "bearer token lowercase",
			authorization: "bearer def456",
			expectedToken: "def456",
		},
		{
			name:          "No Bearer prefix",
			authorization: "xyz789",
			expectedToken: "xyz789",
		},
		{
			name:          "Empty authorization",
			authorization: "",
			expectedToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}

			token := targetcluster.GetToken(req)
			if token != tt.expectedToken {
				t.Errorf("expected token %q, got %q", tt.expectedToken, token)
			}
		})
	}
}

func TestIsIntrospectionQuery(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		{
			name:     "Schema introspection",
			body:     `{"query": "{ __schema { types { name } } }"}`,
			expected: true,
		},
		{
			name:     "Type introspection",
			body:     `{"query": "{ __type(name: \"User\") { name } }"}`,
			expected: true,
		},
		{
			name:     "Normal query",
			body:     `{"query": "{ users { name } }"}`,
			expected: false,
		},
		{
			name:     "Invalid JSON",
			body:     `invalid json`,
			expected: false,
		},
		{
			name:     "Empty body",
			body:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			result := targetcluster.IsIntrospectionQuery(req)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNewGraphQLServer(t *testing.T) {
	log, err := logger.New(logger.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	appCfg := appConfig.Config{}

	server := targetcluster.NewGraphQLServer(log, appCfg)

	if server == nil {
		t.Error("expected non-nil server")
	}
}

func TestCreateHandler(t *testing.T) {
	log, err := logger.New(logger.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	appCfg := appConfig.Config{}
	appCfg.Gateway.HandlerCfg.Pretty = true
	appCfg.Gateway.HandlerCfg.Playground = false
	appCfg.Gateway.HandlerCfg.GraphiQL = true

	server := targetcluster.NewGraphQLServer(log, appCfg)

	// Create a simple test schema
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"hello": &graphql.Field{
					Type: graphql.String,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return "world", nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	handler := server.CreateHandler(&schema)

	if handler == nil {
		t.Error("expected non-nil handler")
		return
	}
	if handler.Schema == nil {
		t.Error("expected non-nil schema in handler")
	}
	if handler.Handler == nil {
		t.Error("expected non-nil HTTP handler")
	}
}

func TestSetContexts(t *testing.T) {
	tests := []struct {
		name      string
		workspace string
		token     string
		enableKcp bool
		expectKcp bool
	}{
		{
			name:      "KCP enabled",
			workspace: "test-workspace",
			token:     "test-token",
			enableKcp: true,
			expectKcp: true,
		},
		{
			name:      "KCP disabled",
			workspace: "test-workspace",
			token:     "test-token",
			enableKcp: false,
			expectKcp: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			result := targetcluster.SetContexts(req, tt.workspace, tt.token, tt.enableKcp)

			// Check token context
			tokenFromCtx := result.Context().Value(roundtripper.TokenKey{})
			if tokenFromCtx != tt.token {
				t.Errorf("expected token %q in context, got %q", tt.token, tokenFromCtx)
			}

			// Check KCP context
			if tt.expectKcp {
				clusterFromCtx, _ := kontext.ClusterFrom(result.Context())
				if clusterFromCtx != logicalcluster.Name(tt.workspace) {
					t.Errorf("expected cluster %q in context, got %q", tt.workspace, clusterFromCtx)
				}
			}
		})
	}
}

func TestHandleSubscription_ErrorCases(t *testing.T) {
	log, err := logger.New(logger.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	appCfg := appConfig.Config{}
	server := targetcluster.NewGraphQLServer(log, appCfg)

	// Create a simple test schema
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"hello": &graphql.Field{
					Type: graphql.String,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return "world", nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
	}{
		{
			name:           "Invalid JSON body",
			requestBody:    `{invalid json}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Empty body",
			requestBody:    ``,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(tt.requestBody)))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			server.HandleSubscription(w, req, &schema)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestHandleSubscription_Headers(t *testing.T) {
	log, err := logger.New(logger.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	appCfg := appConfig.Config{}
	server := targetcluster.NewGraphQLServer(log, appCfg)

	// Create a simple test schema
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"hello": &graphql.Field{
					Type: graphql.String,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return "world", nil
					},
				},
			},
		}),
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name: "Subscription",
			Fields: graphql.Fields{
				"hello": &graphql.Field{
					Type: graphql.String,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return "world", nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"query": "subscription { hello }"}`)))
	req.Header.Set("Content-Type", "application/json")

	// Use context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	go server.HandleSubscription(w, req, &schema)

	// Give it a moment to set headers
	time.Sleep(10 * time.Millisecond)

	// Check SSE headers are set
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("expected Cache-Control no-cache, got %s", w.Header().Get("Cache-Control"))
	}
	if w.Header().Get("Connection") != "keep-alive" {
		t.Errorf("expected Connection keep-alive, got %s", w.Header().Get("Connection"))
	}
}

func TestHandleSubscription_SubscriptionLoop(t *testing.T) {
	log, err := logger.New(logger.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	appCfg := appConfig.Config{}
	server := targetcluster.NewGraphQLServer(log, appCfg)

	// Create schema with subscription that returns data
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"hello": &graphql.Field{
					Type: graphql.String,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return "world", nil
					},
				},
			},
		}),
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name: "Subscription",
			Fields: graphql.Fields{
				"counter": &graphql.Field{
					Type: graphql.Int,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return 42, nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"query": "subscription { counter }"}`)))
	req.Header.Set("Content-Type", "application/json")

	// Use context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	go server.HandleSubscription(w, req, &schema)

	// Give it time to process the subscription
	time.Sleep(100 * time.Millisecond)

	// Check that response was written
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

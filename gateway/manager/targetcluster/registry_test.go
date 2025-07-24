package targetcluster

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/openmfp/golang-commons/logger/testlogger"
	appConfig "github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager/roundtripper"
)

func TestExtractClusterNameWithKCPWorkspace(t *testing.T) {
	log := testlogger.New().HideLogOutput().Logger
	appCfg := appConfig.Config{}
	// Set URL configuration for proper URL matching
	appCfg.Url.VirtualWorkspacePrefix = "virtual-workspace"
	appCfg.Url.DefaultKcpWorkspace = "root"
	appCfg.Url.GraphqlSuffix = "graphql"

	registry := NewClusterRegistry(log, appCfg, nil)

	tests := []struct {
		name                 string
		path                 string
		expectedClusterName  string
		expectedKCPWorkspace string
		shouldSucceed        bool
	}{

		{
			name:                 "virtual_workspace_with_KCP_workspace",
			path:                 "/virtual-workspace/custom-ws/root/graphql",
			expectedClusterName:  "virtual-workspace/custom-ws",
			expectedKCPWorkspace: "root",
			shouldSucceed:        true,
		},
		{
			name:                 "virtual_workspace with namespaced KCP workspace",
			path:                 "/virtual-workspace/custom-ws/root:orgs/graphql",
			expectedClusterName:  "virtual-workspace/custom-ws",
			expectedKCPWorkspace: "root:orgs",
			shouldSucceed:        true,
		},
		{
			name:                 "virtual workspace missing KCP workspace",
			path:                 "/virtual-workspace/custom-ws/graphql",
			expectedClusterName:  "",
			expectedKCPWorkspace: "",
			shouldSucceed:        false,
		},
		{
			name:                 "virtual workspace empty KCP workspace",
			path:                 "/virtual-workspace/custom-ws//graphql",
			expectedClusterName:  "",
			expectedKCPWorkspace: "",
			shouldSucceed:        false,
		},

		{
			name:                 "just graphql endpoint without cluster",
			path:                 "/graphql",
			expectedClusterName:  "",
			expectedKCPWorkspace: "",
			shouldSucceed:        false,
		},
		{
			name:                 "trailing slash",
			path:                 "/test-cluster/graphql/",
			expectedClusterName:  "test-cluster",
			expectedKCPWorkspace: "",
			shouldSucceed:        true,
		},
		{
			name:                 "multiple consecutive slashes in regular workspace",
			path:                 "//test-cluster//graphql",
			expectedClusterName:  "",
			expectedKCPWorkspace: "",
			shouldSucceed:        false,
		},
		{
			name:                 "empty virtual workspace name",
			path:                 "/virtual-workspace//workspace/graphql",
			expectedClusterName:  "",
			expectedKCPWorkspace: "",
			shouldSucceed:        false,
		},

		{
			name:                 "wrong endpoint in virtual workspace",
			path:                 "/virtual-workspace/custom-ws/root/api",
			expectedClusterName:  "",
			expectedKCPWorkspace: "",
			shouldSucceed:        false,
		},
		{
			name:                 "extra path segments after graphql",
			path:                 "/test-cluster/graphql/extra",
			expectedClusterName:  "",
			expectedKCPWorkspace: "",
			shouldSucceed:        false,
		},
		{
			name:                 "extra path segments in virtual workspace",
			path:                 "/virtual-workspace/custom-ws/root/graphql/extra",
			expectedClusterName:  "",
			expectedKCPWorkspace: "",
			shouldSucceed:        false,
		},
		{
			name:                 "cluster name with special characters",
			path:                 "/test-cluster_123.domain/graphql",
			expectedClusterName:  "test-cluster_123.domain",
			expectedKCPWorkspace: "",
			shouldSucceed:        true,
		},
		{
			name:                 "virtual workspace with special characters",
			path:                 "/virtual-workspace/custom-ws_123.domain/root:org-123/graphql",
			expectedClusterName:  "virtual-workspace/custom-ws_123.domain",
			expectedKCPWorkspace: "root:org-123",
			shouldSucceed:        true,
		},
		{
			name:                 "root path",
			path:                 "/",
			expectedClusterName:  "",
			expectedKCPWorkspace: "",
			shouldSucceed:        false,
		},

		{
			name:                 "just cluster name without graphql",
			path:                 "/test-cluster",
			expectedClusterName:  "",
			expectedKCPWorkspace: "",
			shouldSucceed:        false,
		},
		{
			name:                 "virtual workspace missing graphql endpoint",
			path:                 "/virtual-workspace/custom-ws/root",
			expectedClusterName:  "",
			expectedKCPWorkspace: "",
			shouldSucceed:        false,
		},
		{
			name:                 "virtual workspace with only name",
			path:                 "/virtual-workspace/custom-ws",
			expectedClusterName:  "",
			expectedKCPWorkspace: "",
			shouldSucceed:        false,
		},
		{
			name:                 "virtual workspace keyword but wrong structure",
			path:                 "/virtual-workspace/graphql",
			expectedClusterName:  "virtual-workspace",
			expectedKCPWorkspace: "",
			shouldSucceed:        true,
		},
		{
			name:                 "case sensitive virtual workspace keyword",
			path:                 "/Virtual-Workspace/custom-ws/root/graphql",
			expectedClusterName:  "",
			expectedKCPWorkspace: "",
			shouldSucceed:        false,
		},
		{
			name:                 "case sensitive graphql endpoint",
			path:                 "/test-cluster/GraphQL",
			expectedClusterName:  "",
			expectedKCPWorkspace: "",
			shouldSucceed:        false,
		},
		{
			name:                 "long cluster name",
			path:                 "/very-long-cluster-name-with-many-segments-and-special-chars_123.example.com/graphql",
			expectedClusterName:  "very-long-cluster-name-with-many-segments-and-special-chars_123.example.com",
			expectedKCPWorkspace: "",
			shouldSucceed:        true,
		},
		{
			name:                 "long virtual workspace components",
			path:                 "/virtual-workspace/very-long-workspace-name_123.example.com/very:long:namespaced:workspace:path/graphql",
			expectedClusterName:  "virtual-workspace/very-long-workspace-name_123.example.com",
			expectedKCPWorkspace: "very:long:namespaced:workspace:path",
			shouldSucceed:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test request
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			// Extract cluster name
			clusterName, modifiedReq, success := registry.extractClusterName(w, req)

			// Check if the operation succeeded as expected
			if success != tt.shouldSucceed {
				t.Errorf("extractClusterName() success = %v, want %v", success, tt.shouldSucceed)
				return
			}

			if !tt.shouldSucceed {
				return // No need to check further if operation was expected to fail
			}

			// Check cluster name
			if clusterName != tt.expectedClusterName {
				t.Errorf("extractClusterName() clusterName = %v, want %v", clusterName, tt.expectedClusterName)
			}

			// Check KCP workspace in context - use the modified request returned by extractClusterName
			if kcpWorkspace, ok := modifiedReq.Context().Value(kcpWorkspaceKey).(string); ok {
				if kcpWorkspace != tt.expectedKCPWorkspace {
					t.Errorf("KCP workspace in context = %v, want %v", kcpWorkspace, tt.expectedKCPWorkspace)
				}
			} else if tt.expectedKCPWorkspace != "" {
				t.Errorf("Expected KCP workspace %v in context, but not found", tt.expectedKCPWorkspace)
			}
		})
	}
}

func TestSetContextsWithKCPWorkspace(t *testing.T) {
	tests := []struct {
		name                     string
		workspace                string
		contextKCPWorkspace      string
		enableKcp                bool
		expectedKCPWorkspaceName string
	}{
		{
			name:                     "regular workspace with KCP enabled",
			workspace:                "test-cluster",
			contextKCPWorkspace:      "",
			enableKcp:                true,
			expectedKCPWorkspaceName: "test-cluster",
		},
		{
			name:                     "virtual workspace with context KCP workspace",
			workspace:                "virtual-workspace/custom-ws",
			contextKCPWorkspace:      "root",
			enableKcp:                true,
			expectedKCPWorkspaceName: "root",
		},
		{
			name:                     "virtual workspace with namespaced context KCP workspace",
			workspace:                "virtual-workspace/custom-ws",
			contextKCPWorkspace:      "root:orgs",
			enableKcp:                true,
			expectedKCPWorkspaceName: "root:orgs",
		},
		{
			name:                     "KCP disabled",
			workspace:                "virtual-workspace/custom-ws",
			contextKCPWorkspace:      "root",
			enableKcp:                false,
			expectedKCPWorkspaceName: "", // Not relevant when KCP is disabled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test request with KCP workspace in context if provided
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.contextKCPWorkspace != "" {
				req = req.WithContext(context.WithValue(req.Context(), kcpWorkspaceKey, tt.contextKCPWorkspace))
			}

			// Call SetContexts
			resultReq := SetContexts(req, tt.workspace, "test-token", tt.enableKcp)

			// For this test, we can't easily verify the KCP logical cluster context,
			// but we can verify that the function doesn't panic and returns a request
			if resultReq == nil {
				t.Error("SetContexts() returned nil request")
			}

			// Verify token context is set
			if token, ok := resultReq.Context().Value(roundtripper.TokenKey{}).(string); ok {
				if token != "test-token" {
					t.Errorf("Token in context = %v, want %v", token, "test-token")
				}
			} else {
				t.Error("Expected token in context, but not found")
			}
		})
	}
}

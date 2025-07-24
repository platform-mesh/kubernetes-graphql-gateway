package targetcluster_test

import (
	"testing"

	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager/targetcluster"
)

func TestMatchURL(t *testing.T) {
	tests := []struct {
		name                 string
		path                 string
		expectedCluster      string
		expectedKCPWorkspace string
		expectedValid        bool
	}{
		{
			name:                 "regular_workspace_pattern",
			path:                 "/test-cluster/graphql",
			expectedCluster:      "test-cluster",
			expectedKCPWorkspace: "",
			expectedValid:        true,
		},
		{
			name:                 "virtual_workspace_pattern",
			path:                 "/virtual-workspace/my-workspace/root/graphql",
			expectedCluster:      "virtual-workspace/my-workspace",
			expectedKCPWorkspace: "root",
			expectedValid:        true,
		},
		{
			name:                 "virtual_workspace_with_complex_names",
			path:                 "/virtual-workspace/complex-ws_123.domain/root:org:team/graphql",
			expectedCluster:      "virtual-workspace/complex-ws_123.domain",
			expectedKCPWorkspace: "root:org:team",
			expectedValid:        true,
		},
		{
			name:                 "invalid_path",
			path:                 "/invalid/path/structure",
			expectedCluster:      "",
			expectedKCPWorkspace: "",
			expectedValid:        false,
		},
		{
			name:                 "missing_graphql_endpoint",
			path:                 "/test-cluster/api",
			expectedCluster:      "",
			expectedKCPWorkspace: "",
			expectedValid:        false,
		},
		{
			name:                 "empty_cluster_name",
			path:                 "//graphql",
			expectedCluster:      "",
			expectedKCPWorkspace: "",
			expectedValid:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{}
			cfg.Url.VirtualWorkspacePrefix = "virtual-workspace"
			cfg.Url.GraphqlSuffix = "graphql"

			clusterName, kcpWorkspace, valid := targetcluster.MatchURL(tt.path, cfg)

			if valid != tt.expectedValid {
				t.Errorf("Match() valid = %v, want %v", valid, tt.expectedValid)
				return
			}

			if !tt.expectedValid {
				return
			}

			if clusterName != tt.expectedCluster {
				t.Errorf("Match() clusterName = %v, want %v", clusterName, tt.expectedCluster)
			}

			if kcpWorkspace != tt.expectedKCPWorkspace {
				t.Errorf("Match() kcpWorkspace = %v, want %v", kcpWorkspace, tt.expectedKCPWorkspace)
			}
		})
	}
}

package options

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

func TestApplyLogicalClusterToConfig(t *testing.T) {
	tests := []struct {
		name           string
		host           string
		logicalCluster string
		wantHost       string
		wantErr        bool
	}{
		{
			name:           "empty logical cluster returns cfg unchanged",
			host:           "https://kcp.example.com/clusters/root",
			logicalCluster: "",
			wantHost:       "https://kcp.example.com/clusters/root",
		},
		{
			name:           "rewrites host path to logical cluster",
			host:           "https://kcp.example.com/clusters/root",
			logicalCluster: "root:providers",
			wantHost:       "https://kcp.example.com/clusters/root:providers",
		},
		{
			name:           "rewrites host with no path",
			host:           "https://kcp.example.com",
			logicalCluster: "root:providers",
			wantHost:       "https://kcp.example.com/clusters/root:providers",
		},
		{
			name:           "invalid host url returns error",
			host:           "://bad",
			logicalCluster: "root:providers",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &CompletedOptions{
				completedOptions: &completedOptions{
					ExtraOptions: ExtraOptions{
						APIExportEndpointSliceLogicalCluster: tt.logicalCluster,
					},
				},
			}

			cfg := &rest.Config{Host: tt.host}
			got, err := opts.ApplyLogicalClusterToConfig(cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", got.Host, tt.wantHost)
			}

			if tt.logicalCluster != "" && got == cfg {
				t.Errorf("expected copy of rest.Config, got same pointer")
			}
			if tt.logicalCluster == "" && got != cfg {
				t.Errorf("expected unchanged rest.Config to be returned as-is")
			}
		})
	}
}

func TestGetClusterMetadataOverrideFunc_perClusterHostWithoutTokenReviewHost(t *testing.T) {
	opts, err := (&Options{
		ExtraOptions: ExtraOptions{
			WorkspaceSchemaKubeconfigRestConfig: &rest.Config{
				Host: "https://kcp.example/clusters/root:platform-mesh-system",
			},
		},
	}).Complete()
	require.NoError(t, err)

	override := opts.GetClusterMetadataOverrideFunc()
	metadata, err := override("root:orgs:org1:account1")
	require.NoError(t, err)
	require.NotNil(t, metadata)

	assert.Equal(t, "https://kcp.example/clusters/root:orgs:org1:account1", metadata.Host)
	assert.Empty(t, metadata.TokenReviewHost, "TokenReview must use per-cluster path injection, not a fixed home workspace host")
}

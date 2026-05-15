package options

import (
	"testing"

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

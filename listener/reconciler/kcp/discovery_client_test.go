package kcp_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"

	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler/kcp"
	kcpmocks "github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler/kcp/mocks"
)

func TestNewDiscoveryFactory(t *testing.T) {
	tests := []struct {
		name        string
		config      *rest.Config
		wantErr     bool
		errContains string
	}{
		{
			name: "successful_creation",
			config: &rest.Config{
				Host: "https://api.example.com",
			},
			wantErr: false,
		},
		{
			name:        "nil_config_returns_error",
			config:      nil,
			wantErr:     true,
			errContains: "config cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := kcp.NewDiscoveryFactoryExported(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.config, got.Config)
				assert.NotNil(t, got.NewDiscoveryIFFunc)
			}
		})
	}
}

func TestDiscoveryFactoryProvider_ClientForCluster(t *testing.T) {
	baseConfig := &rest.Config{
		Host: "https://api.example.com",
	}

	tests := []struct {
		name               string
		clusterName        string
		newDiscoveryIFFunc func(cfg *rest.Config) (discovery.DiscoveryInterface, error)
		wantErr            bool
		errContains        string
		expectedConfigHost string
	}{
		{
			name:        "successful_client_creation",
			clusterName: "test-cluster",
			newDiscoveryIFFunc: func(cfg *rest.Config) (discovery.DiscoveryInterface, error) {
				// Verify the config was properly modified for the cluster
				assert.Equal(t, "https://api.example.com/clusters/test-cluster", cfg.Host)
				return kcpmocks.NewMockDiscoveryInterface(t), nil
			},
			wantErr:            false,
			expectedConfigHost: "https://api.example.com/clusters/test-cluster",
		},
		{
			name:        "discovery_client_creation_error",
			clusterName: "test-cluster",
			newDiscoveryIFFunc: func(cfg *rest.Config) (discovery.DiscoveryInterface, error) {
				return nil, errors.New("discovery client creation failed")
			},
			wantErr:     true,
			errContains: "discovery client creation failed",
		},
		{
			name:        "config_parsing_error_in_cluster_config",
			clusterName: "test-cluster",
			newDiscoveryIFFunc: func(cfg *rest.Config) (discovery.DiscoveryInterface, error) {
				// This should not be called if ConfigForKCPCluster fails
				t.Fatal("NewDiscoveryIFFunc should not be called when ConfigForKCPCluster fails")
				return nil, nil
			},
			wantErr:     true,
			errContains: "failed to get rest config for cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := baseConfig
			if tt.name == "config_parsing_error_in_cluster_config" {
				// Use an invalid config to trigger ConfigForKCPCluster error
				config = &rest.Config{Host: "://invalid-url"}
			}

			factory := &kcp.ExportedDiscoveryFactoryProvider{
				Config:             config,
				NewDiscoveryIFFunc: tt.newDiscoveryIFFunc,
			}

			got, err := factory.ClientForCluster(tt.clusterName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}

func TestDiscoveryFactoryProvider_RestMapperForCluster(t *testing.T) {
	baseConfig := &rest.Config{
		Host: "https://api.example.com",
		// Add minimal required config for HTTP client creation
		TLSClientConfig: rest.TLSClientConfig{Insecure: true},
	}

	tests := []struct {
		name        string
		clusterName string
		config      *rest.Config
		wantErr     bool
		errContains string
	}{
		{
			name:        "successful_rest_mapper_creation",
			clusterName: "test-cluster",
			config:      baseConfig,
			wantErr:     false,
		},
		{
			name:        "config_parsing_error",
			clusterName: "test-cluster",
			config:      &rest.Config{Host: "://invalid-url"},
			wantErr:     true,
			errContains: "failed to get rest config for cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := &kcp.ExportedDiscoveryFactoryProvider{
				Config: tt.config,
			}

			got, err := factory.RestMapperForCluster(tt.clusterName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Implements(t, (*meta.RESTMapper)(nil), got)
			}
		})
	}
}

func TestDiscoveryConstants(t *testing.T) {
	t.Run("error_variables", func(t *testing.T) {
		assert.Equal(t, "config cannot be nil", kcp.ErrNilDiscoveryConfigExported.Error())
		assert.Equal(t, "failed to get rest config for cluster", kcp.ErrGetDiscoveryClusterConfigExported.Error())
		assert.Equal(t, "failed to parse rest config's Host URL", kcp.ErrParseDiscoveryHostURLExported.Error())
		assert.Equal(t, "failed to create http client", kcp.ErrCreateHTTPClientExported.Error())
		assert.Equal(t, "failed to create dynamic REST mapper", kcp.ErrCreateDynamicMapperExported.Error())
	})
}

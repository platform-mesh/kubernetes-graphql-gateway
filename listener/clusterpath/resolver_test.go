package clusterpath

import (
	"net/url"
	"testing"

	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolver(t *testing.T) {
	tests := map[string]struct {
		baseConfig  *rest.Config
		clusterName string
		expectErr   bool
	}{
		"valid_cluster":   {baseConfig: &rest.Config{}, clusterName: "test-cluster", expectErr: false},
		"nil_base_config": {baseConfig: nil, clusterName: "test-cluster", expectErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			resolver := &Resolver{
				Scheme: runtime.NewScheme(),
				Config: tc.baseConfig,
				clientFactory: func(config *rest.Config, options client.Options) (client.Client, error) {
					return fake.NewClientBuilder().WithScheme(options.Scheme).Build(), nil
				},
			}

			client, err := resolver.ClientForCluster(tc.clusterName)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, client)

		})
	}
}

func TestPathForCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	err := kcpcore.AddToScheme(scheme)
	assert.NoError(t, err)
	tests := map[string]struct {
		clusterName  string
		annotations  map[string]string
		expectErr    bool
		expectedPath string
	}{
		"root_cluster": {
			clusterName:  "root",
			annotations:  nil,
			expectErr:    false,
			expectedPath: "root",
		},
		"valid_cluster_with_1st_level_path": {
			clusterName:  "sap",
			annotations:  map[string]string{"kcp.io/path": "root:sap"},
			expectErr:    false,
			expectedPath: "root:sap",
		},
		"valid_cluster_with_2nd_level_path": {
			clusterName:  "openmfp",
			annotations:  map[string]string{"kcp.io/path": "root:sap:openmfp"},
			expectErr:    false,
			expectedPath: "root:sap:openmfp",
		},
		"missing_annotation": {
			clusterName: "test-cluster",
			annotations: map[string]string{},
			expectErr:   true,
		},
		"nil_annotation": {
			clusterName: "test-cluster",
			annotations: nil,
			expectErr:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme)
			if tc.annotations != nil {
				lc := &kcpcore.LogicalCluster{}
				lc.SetName("cluster")
				lc.SetAnnotations(tc.annotations)
				builder = builder.WithObjects(lc)
			}
			clt := builder.Build()

			path, err := PathForCluster(tc.clusterName, clt)
			if tc.expectErr {
				assert.Error(t, err)
				assert.Empty(t, path)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedPath, path)

		})
	}
}

func TestGetClusterConfig(t *testing.T) {
	tests := map[string]struct {
		cfg       *rest.Config
		cluster   string
		expect    *rest.Config
		expectErr bool
	}{
		"nil_config": {
			cfg:       nil,
			cluster:   "openmfp",
			expect:    nil,
			expectErr: true,
		},
		"valid_config": {
			cfg:       &rest.Config{Host: "https://127.0.0.1:56120/clusters/root"},
			cluster:   "openmfp",
			expect:    &rest.Config{Host: "https://127.0.0.1:56120/clusters/openmfp"},
			expectErr: false,
		},
		"invalid_URL": {
			cfg:       &rest.Config{Host: ":://bad-url"},
			cluster:   "openmfp",
			expect:    nil,
			expectErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := getClusterConfig(tc.cluster, tc.cfg)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, tc.expect.Host, got.Host)
			parsedURL, err1 := url.Parse(got.Host)
			assert.NoError(t, err1)
			assert.NotEmpty(t, parsedURL)
			expectedURL, err2 := url.Parse(tc.expect.Host)
			assert.NoError(t, err2)
			assert.NotEmpty(t, expectedURL)
			assert.Equal(t, expectedURL, parsedURL)
		})
	}
}

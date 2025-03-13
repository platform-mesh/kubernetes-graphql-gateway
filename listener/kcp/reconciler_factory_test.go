package kcp

import (
	"errors"
	"path"
	"testing"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/openmfp/kubernetes-graphql-gateway/listener/apischema"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/discoveryclient"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/workspacefile"
)

func TestNewReconciler(t *testing.T) {
	tempDir := t.TempDir()

	tests := map[string]struct {
		cfg             *rest.Config
		definitionsPath string
		isKCPEnabled    bool
		expectErr       bool
	}{
		"standard_reconciler_creation": {
			cfg:             &rest.Config{Host: validAPIServerHost},
			definitionsPath: tempDir,
			isKCPEnabled:    false,
			expectErr:       false,
		},
		"kcp_reconciler_creation": {
			cfg:             &rest.Config{Host: validAPIServerHost},
			definitionsPath: tempDir,
			isKCPEnabled:    true,
			expectErr:       false,
		},
		"failure_in_discovery_client_creation": {
			cfg:             nil,
			definitionsPath: tempDir,
			isKCPEnabled:    false,
			expectErr:       true,
		},
		"success_in_non-existent-dir": {
			cfg:             &rest.Config{Host: validAPIServerHost},
			definitionsPath: path.Join(tempDir, "non-existent"),
			isKCPEnabled:    false,
			expectErr:       false,
		},
		"failure_in_rest_mapper_creation": {
			cfg:             &rest.Config{Host: schemelessAPIServerHost},
			definitionsPath: tempDir,
			isKCPEnabled:    false,
			expectErr:       true,
		},
		"failure_in_virtual_workspace_config_retrieval_(kcp)": {
			cfg:             &rest.Config{Host: schemelessAPIServerHost},
			definitionsPath: tempDir,
			isKCPEnabled:    true,
			expectErr:       true,
		},
		"failure_in_kcp_discovery_client_factory_creation": {
			cfg:             nil,
			definitionsPath: tempDir,
			isKCPEnabled:    true,
			expectErr:       true,
		},
		"failure_in_cluster_path_resolver_creation": {
			cfg:             &rest.Config{Host: schemelessAPIServerHost},
			definitionsPath: tempDir,
			isKCPEnabled:    true,
			expectErr:       true,
		},
	}

	for name, tc := range tests {
		scheme := runtime.NewScheme()
		err := kcpapis.AddToScheme(scheme)
		assert.NoError(t, err)
		t.Run(name, func(t *testing.T) {

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects([]client.Object{
				&kcpapis.APIExport{
					ObjectMeta: metav1.ObjectMeta{Name: tenancyAPIExportName},
					Status: kcpapis.APIExportStatus{
						VirtualWorkspaces: []kcpapis.VirtualWorkspace{
							{URL: validAPIServerHost},
						},
					},
				},
			}...).Build()
			f := &ReconcilerFactory{
				IsKCPEnabled:       tc.isKCPEnabled,
				newDiscoveryIFFunc: fakeClientFactory,
				preReconcileFunc: func(cr *apischema.CRDResolver, io *workspacefile.IOHandler) error {
					return nil
				},
				newDiscoveryFactoryFunc: func(cfg *rest.Config) (*discoveryclient.Factory, error) {
					return &discoveryclient.Factory{
						Config:             cfg,
						NewDiscoveryIFFunc: fakeClientFactory,
					}, nil
				},
			}
			reconciler, err := f.NewReconciler(ReconcilerOpts{
				Config:                 tc.cfg,
				Scheme:                 scheme,
				Client:                 fakeClient,
				OpenAPIDefinitionsPath: tc.definitionsPath,
			})

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, reconciler)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, reconciler)
		})
	}
}

func fakeClientFactory(cfg *rest.Config) (discovery.DiscoveryInterface, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}
	client := fakeclientset.NewClientset()
	fakeDiscovery, ok := client.Discovery().(*fakediscovery.FakeDiscovery)
	if !ok {
		return nil, errors.New("failed to get fake discovery client")
	}
	return fakeDiscovery, nil
}

package kcp

import (
	"context"
	"errors"
	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/clusterpath"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/kcp/mocks"
	"github.com/stretchr/testify/require"
	"path"
	"testing"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/openmfp/kubernetes-graphql-gateway/listener/apischema"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/discoveryclient"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/workspacefile"
)

const (
	validAPIServerHost      = "https://192.168.1.13:6443"
	schemalessAPIServerHost = "://192.168.1.13:6443"
)

func TestNewReconciler(t *testing.T) {
	tempDir := t.TempDir()

	tests := map[string]struct {
		cfg             *rest.Config
		definitionsPath string
		isKCPEnabled    bool
		err             error
	}{
		"standard_reconciler_creation": {
			cfg:             &rest.Config{Host: validAPIServerHost},
			definitionsPath: tempDir,
			isKCPEnabled:    false,
		},
		"kcp_reconciler_creation": {
			cfg:             &rest.Config{Host: validAPIServerHost},
			definitionsPath: tempDir,
			isKCPEnabled:    true,
		},
		"failure_in_creation_cluster_path_resolver_due_to_nil_config_with_kcp_enabled": {
			cfg:             nil,
			definitionsPath: tempDir,
			isKCPEnabled:    true,
			err:             errors.Join(ErrCreatePathResolver, clusterpath.ErrNilConfig),
		},
		"success_in_non-existent-dir": {
			cfg:             &rest.Config{Host: validAPIServerHost},
			definitionsPath: path.Join(tempDir, "non-existent"),
			isKCPEnabled:    false,
		},
		"failure_in_rest_mapper_creation": {
			cfg:             &rest.Config{Host: schemalessAPIServerHost},
			definitionsPath: tempDir,
			isKCPEnabled:    false,
			err:             errors.Join(ErrCreateRestMapper, errors.New("host must be a URL or a host:port pair: \"://192.168.1.13:6443\"")),
		},
		"failure_in_definition_dir_creation": {
			cfg:             &rest.Config{Host: validAPIServerHost},
			definitionsPath: "/dev/null/schemas",
			isKCPEnabled:    false,
			err:             errors.Join(ErrCreateIOHandler, workspacefile.ErrCreateSchemasDir, errors.New("mkdir /dev/null: not a directory")),
		},
	}

	log, err := logger.New(logger.DefaultConfig())
	require.NoError(t, err)

	for name, tc := range tests {
		scheme := runtime.NewScheme()
		assert.NoError(t, kcpapis.AddToScheme(scheme))

		t.Run(name, func(t *testing.T) {
			appCfg, err := config.NewFromEnv()
			assert.NoError(t, err)
			appCfg.EnableKcp = tc.isKCPEnabled

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects([]client.Object{
				&kcpapis.APIExport{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: appCfg.ApiExportWorkspace,
						Name:      appCfg.ApiExportName,
					},
					Status: kcpapis.APIExportStatus{
						VirtualWorkspaces: []kcpapis.VirtualWorkspace{
							{URL: validAPIServerHost},
						},
					},
				},
			}...).Build()

			reconciler, err := NewReconciler(
				context.Background(),
				log,
				appCfg,
				ReconcilerOpts{
					Config:                 tc.cfg,
					Scheme:                 scheme,
					Client:                 fakeClient,
					OpenAPIDefinitionsPath: tc.definitionsPath,
				},
				&mocks.MockDiscoveryInterface{},
				func(cr *apischema.CRDResolver, io *workspacefile.IOHandler) error {
					return nil
				},
				func(cfg *rest.Config) (*discoveryclient.Factory, error) {
					return &discoveryclient.Factory{
						Config: cfg,
					}, nil
				})

			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error())
				assert.Nil(t, reconciler)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, reconciler)
			}
		})
	}
}

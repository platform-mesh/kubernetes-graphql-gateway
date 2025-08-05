package clusteraccess_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmfp/golang-commons/logger"
	gatewayv1alpha1 "github.com/openmfp/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/common/mocks"
	apischema_mocks "github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/apischema/mocks"
	workspacefile_mocks "github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/workspacefile/mocks"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler/clusteraccess"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestCheckClusterAccessCRDStatus(t *testing.T) {
	mockLogger, _ := logger.New(logger.DefaultConfig())

	tests := []struct {
		name      string
		mockSetup func(*mocks.MockClient)
		want      clusteraccess.CRDStatus
		wantErr   bool
	}{
		{
			name: "CRD_registered_and_available",
			mockSetup: func(m *mocks.MockClient) {
				m.EXPECT().List(mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterAccessList")).
					RunAndReturn(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						clusterAccessList := list.(*gatewayv1alpha1.ClusterAccessList)
						clusterAccessList.Items = []gatewayv1alpha1.ClusterAccess{
							{
								ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
								Spec: gatewayv1alpha1.ClusterAccessSpec{
									Host: "https://test.example.com",
								},
							},
						}
						return nil
					}).Once()
			},
			want:    clusteraccess.CRDRegistered,
			wantErr: false,
		},

		{
			name: "CRD_not_registered_-_NoMatchError",
			mockSetup: func(m *mocks.MockClient) {
				m.EXPECT().List(mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterAccessList")).
					Return(&meta.NoResourceMatchError{
						PartialResource: schema.GroupVersionResource{
							Group:    "gateway.openmfp.org",
							Version:  "v1alpha1",
							Resource: "clusteraccesses",
						},
					}).Once()
			},
			want:    clusteraccess.CRDNotRegistered,
			wantErr: false,
		},
		{
			name: "API_server_error",
			mockSetup: func(m *mocks.MockClient) {
				m.EXPECT().List(mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterAccessList")).
					Return(errors.New("API server connection failed")).Once()
			},
			want:    clusteraccess.CRDNotRegistered,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocks.NewMockClient(t)
			tt.mockSetup(mockClient)

			crdStatus, err := clusteraccess.CheckClusterAccessCRDStatus(t.Context(), mockClient, mockLogger)
			_ = err

			assert.Equal(t, tt.want, crdStatus)
		})
	}
}

func TestNewClusterAccessReconciler(t *testing.T) {
	mockLogger, _ := logger.New(logger.DefaultConfig())

	tests := []struct {
		name        string
		setupMocks  func() *mocks.MockClient
		wantErr     bool
		errContains string
	}{
		{
			name: "success_with_registered_crd",
			setupMocks: func() *mocks.MockClient {
				mockClient := &mocks.MockClient{}
				mockClient.On("List", mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterAccessList")).Return(nil)
				return mockClient
			},
			wantErr: false,
		},
		{
			name: "error_crd_not_registered",
			setupMocks: func() *mocks.MockClient {
				mockClient := &mocks.MockClient{}
				noMatchErr := &meta.NoResourceMatchError{PartialResource: schema.GroupVersionResource{}}
				mockClient.On("List", mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterAccessList")).Return(noMatchErr)
				return mockClient
			},
			wantErr:     true,
			errContains: "ClusterAccess CRD not registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := tt.setupMocks()

			// Test the actual NewClusterAccessReconciler function
			ctx := context.Background()
			appCfg := config.Config{
				OpenApiDefinitionsPath: "/tmp/test",
			}
			opts := reconciler.ReconcilerOpts{
				Config: &rest.Config{Host: "https://test-api-server.com"},
				Scheme: runtime.NewScheme(),
				Client: mockClient,
				ManagerOpts: ctrl.Options{
					Scheme: runtime.NewScheme(),
				},
				OpenAPIDefinitionsPath: "/tmp/test",
			}

			// Create required dependencies using mocks
			mockIOHandler := &workspacefile_mocks.MockIOHandler{}
			mockSchemaResolver := &apischema_mocks.MockResolver{}

			reconciler, err := clusteraccess.NewClusterAccessReconciler(
				ctx,
				appCfg,
				opts,
				mockIOHandler,
				mockSchemaResolver,
				mockLogger,
			)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, reconciler)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, reconciler)
			}
		})
	}
}

func TestNewClusterAccessReconciler_NilDependencyValidation(t *testing.T) {
	mockLogger, _ := logger.New(logger.DefaultConfig())
	ctx := context.Background()
	appCfg := config.Config{
		OpenApiDefinitionsPath: "/tmp/test",
	}
	opts := reconciler.ReconcilerOpts{
		Config: &rest.Config{Host: "https://test-api-server.com"},
		Scheme: runtime.NewScheme(),
		Client: &mocks.MockClient{},
		ManagerOpts: ctrl.Options{
			Scheme: runtime.NewScheme(),
		},
		OpenAPIDefinitionsPath: "/tmp/test",
	}

	t.Run("nil_ioHandler", func(t *testing.T) {
		mockSchemaResolver := &apischema_mocks.MockResolver{}

		reconciler, err := clusteraccess.NewClusterAccessReconciler(
			ctx,
			appCfg,
			opts,
			nil, // nil ioHandler
			mockSchemaResolver,
			mockLogger,
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ioHandler is required")
		assert.Nil(t, reconciler)
	})

	t.Run("nil_schemaResolver", func(t *testing.T) {
		mockIOHandler := &workspacefile_mocks.MockIOHandler{}

		reconciler, err := clusteraccess.NewClusterAccessReconciler(
			ctx,
			appCfg,
			opts,
			mockIOHandler,
			nil, // nil schemaResolver
			mockLogger,
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "schemaResolver is required")
		assert.Nil(t, reconciler)
	})
}

func TestConstants(t *testing.T) {
	t.Run("error_variables", func(t *testing.T) {
		assert.Equal(t, "ClusterAccess CRD not registered", clusteraccess.ErrCRDNotRegistered.Error())
		assert.Equal(t, "failed to check ClusterAccess CRD status", clusteraccess.ErrCRDCheckFailed.Error())
	})
}

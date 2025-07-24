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
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmfp/golang-commons/logger"
	gatewayv1alpha1 "github.com/openmfp/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/common/mocks"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler/clusteraccess"
)

func TestCheckClusterAccessCRDStatus(t *testing.T) {
	mockLogger, _ := logger.New(logger.DefaultConfig())

	tests := []struct {
		name      string
		mockSetup func(*mocks.MockClient)
		want      clusteraccess.ExportedCRDStatus
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
			want:    clusteraccess.ExportedCRDRegistered,
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
			want:    clusteraccess.ExportedCRDNotRegistered,
			wantErr: false,
		},
		{
			name: "API_server_error",
			mockSetup: func(m *mocks.MockClient) {
				m.EXPECT().List(mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterAccessList")).
					Return(errors.New("API server connection failed")).Once()
			},
			want:    clusteraccess.ExportedCRDNotRegistered,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocks.NewMockClient(t)
			tt.mockSetup(mockClient)

			got, err := clusteraccess.CheckClusterAccessCRDStatus(t.Context(), mockClient, mockLogger)
			_ = err

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCreateMultiClusterReconciler(t *testing.T) {
	mockLogger, _ := logger.New(logger.DefaultConfig())

	tests := []struct {
		name        string
		mockSetup   func(*mocks.MockClient)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful_creation_with_clusteraccess_available",
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
			wantErr: false,
		},
		{
			name: "error_when_CRD_not_registered",
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
			wantErr:     true,
			errContains: "multi-cluster mode enabled but ClusterAccess CRD not registered",
		},
		{
			name: "error_when_CRD_check_fails",
			mockSetup: func(m *mocks.MockClient) {
				m.EXPECT().List(mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterAccessList")).
					Return(errors.New("API server connection failed")).Once()
			},
			wantErr:     true,
			errContains: "failed to check ClusterAccess CRD status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocks.NewMockClient(t)
			tt.mockSetup(mockClient)

			// Create temporary directory for OpenApiDefinitionsPath
			tempDir := t.TempDir()
			opts := reconciler.ReconcilerOpts{
				Client:                 mockClient,
				Config:                 &rest.Config{Host: "https://test.example.com"},
				OpenAPIDefinitionsPath: tempDir,
			}

			testConfig := config.Config{
				OpenApiDefinitionsPath: tempDir,
			}

			reconciler, err := clusteraccess.CreateMultiClusterReconciler(testConfig, opts, mockLogger)

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

func TestConstants(t *testing.T) {
	t.Run("error_variables", func(t *testing.T) {
		assert.Equal(t, "ClusterAccess CRD not registered", clusteraccess.ExportedErrCRDNotRegistered.Error())
		assert.Equal(t, "failed to check ClusterAccess CRD status", clusteraccess.ExportedErrCRDCheckFailed.Error())
	})
}

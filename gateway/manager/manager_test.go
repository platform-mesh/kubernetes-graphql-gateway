package manager

import (
	"errors"
	"testing"

	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager/mocks"
	"github.com/stretchr/testify/assert"
)

func TestService_Close(t *testing.T) {
	tests := []struct {
		name         string
		setupService func(t *testing.T) *Service
		expectError  bool
	}{
		{
			name: "both_services_nil",
			setupService: func(t *testing.T) *Service {
				log, err := logger.New(logger.DefaultConfig())
				assert.NoError(t, err)
				return &Service{
					log:             log,
					clusterRegistry: nil,
					schemaWatcher:   nil,
				}
			},
			expectError: false,
		},
		{
			name: "cluster_registry_nil_schema_watcher_present",
			setupService: func(t *testing.T) *Service {
				log, err := logger.New(logger.DefaultConfig())
				assert.NoError(t, err)

				mockSchema := mocks.NewMockSchemaWatcher(t)
				mockSchema.EXPECT().Close().Return(nil)

				return &Service{
					log:             log,
					clusterRegistry: nil,
					schemaWatcher:   mockSchema,
				}
			},
			expectError: false,
		},
		{
			name: "schema_watcher_nil_cluster_registry_present",
			setupService: func(t *testing.T) *Service {
				log, err := logger.New(logger.DefaultConfig())
				assert.NoError(t, err)

				mockCluster := mocks.NewMockClusterManager(t)
				mockCluster.EXPECT().Close().Return(nil)

				return &Service{
					log:             log,
					clusterRegistry: mockCluster,
					schemaWatcher:   nil,
				}
			},
			expectError: false,
		},
		{
			name: "both_services_present_successful_close",
			setupService: func(t *testing.T) *Service {
				log, err := logger.New(logger.DefaultConfig())
				assert.NoError(t, err)

				mockCluster := mocks.NewMockClusterManager(t)
				mockCluster.EXPECT().Close().Return(nil)

				mockSchema := mocks.NewMockSchemaWatcher(t)
				mockSchema.EXPECT().Close().Return(nil)

				return &Service{
					log:             log,
					clusterRegistry: mockCluster,
					schemaWatcher:   mockSchema,
				}
			},
			expectError: false,
		},
		{
			name: "schema_watcher_close_error_cluster_registry_succeeds",
			setupService: func(t *testing.T) *Service {
				log, err := logger.New(logger.DefaultConfig())
				assert.NoError(t, err)

				mockCluster := mocks.NewMockClusterManager(t)
				mockCluster.EXPECT().Close().Return(nil)

				mockSchema := mocks.NewMockSchemaWatcher(t)
				mockSchema.EXPECT().Close().Return(errors.New("schema watcher close error"))

				return &Service{
					log:             log,
					clusterRegistry: mockCluster,
					schemaWatcher:   mockSchema,
				}
			},
			expectError: false, // Service.Close() doesn't propagate errors
		},
		{
			name: "cluster_registry_close_error_schema_watcher_succeeds",
			setupService: func(t *testing.T) *Service {
				log, err := logger.New(logger.DefaultConfig())
				assert.NoError(t, err)

				mockCluster := mocks.NewMockClusterManager(t)
				mockCluster.EXPECT().Close().Return(errors.New("cluster registry close error"))

				mockSchema := mocks.NewMockSchemaWatcher(t)
				mockSchema.EXPECT().Close().Return(nil)

				return &Service{
					log:             log,
					clusterRegistry: mockCluster,
					schemaWatcher:   mockSchema,
				}
			},
			expectError: false, // Service.Close() doesn't propagate errors
		},
		{
			name: "both_services_close_with_errors",
			setupService: func(t *testing.T) *Service {
				log, err := logger.New(logger.DefaultConfig())
				assert.NoError(t, err)

				mockCluster := mocks.NewMockClusterManager(t)
				mockCluster.EXPECT().Close().Return(errors.New("cluster registry close error"))

				mockSchema := mocks.NewMockSchemaWatcher(t)
				mockSchema.EXPECT().Close().Return(errors.New("schema watcher close error"))

				return &Service{
					log:             log,
					clusterRegistry: mockCluster,
					schemaWatcher:   mockSchema,
				}
			},
			expectError: false, // Service.Close() doesn't propagate errors
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := tt.setupService(t)

			err := service.Close()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

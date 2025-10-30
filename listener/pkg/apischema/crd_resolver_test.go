package apischema_test

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/validation/spec"

	"github.com/platform-mesh/golang-commons/logger/testlogger"
	apischema "github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	apischemaMocks "github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema/mocks"
	"github.com/stretchr/testify/assert"
)

// TestGetSchemaForPath tests the getSchemaForPath function. It checks if the function
// correctly retrieves the schema for a given path and handles various error cases.
func TestGetSchemaForPath(t *testing.T) {
	// prepare a valid schemaResponse JSON
	validSchemas := map[string]*spec.Schema{"a.v1.K": {}}
	resp := apischema.SchemaResponse{Components: apischema.SchemasComponentsWrapper{Schemas: validSchemas}}
	validJSON, err := json.Marshal(&resp)
	assert.NoError(t, err, "failed to marshal valid response")

	tests := []struct {
		name      string
		preferred []string
		path      string
		gv        openapi.GroupVersion
		wantErr   error
		wantCount int
	}{
		{
			name:      "invalid_path",
			preferred: []string{"g/v1"},
			path:      "noSlash",
			gv:        apischemaMocks.NewMockGroupVersion(t),
			wantErr:   apischema.ErrInvalidPath,
		},
		{
			name:      "not_preferred",
			preferred: []string{"x/y"},
			path:      "/g/v1",
			gv:        apischemaMocks.NewMockGroupVersion(t),
			wantErr:   apischema.ErrNotPreferred,
		},
		{
			name:      "unmarshal_error",
			preferred: []string{"g/v1"},
			path:      "/g/v1",
			gv: func() openapi.GroupVersion {
				mock := apischemaMocks.NewMockGroupVersion(t)
				mock.EXPECT().Schema("application/json").Return([]byte("bad json"), nil)
				return mock
			}(),
			wantErr: apischema.ErrUnmarshalSchemaForPath,
		},
		{
			name:      "success",
			preferred: []string{"g/v1"},
			path:      "/g/v1",
			gv: func() openapi.GroupVersion {
				mock := apischemaMocks.NewMockGroupVersion(t)
				mock.EXPECT().Schema("application/json").Return(validJSON, nil)
				return mock
			}(),
			wantCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := apischema.GetSchemaForPath(tc.preferred, tc.path, tc.gv)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantCount, len(got), "schema count mismatch")
		})
	}
}

// TestResolveSchema tests the resolveSchema function. It checks if the function
// correctly resolves the schema for a given CRD and handles various error cases.
func TestResolveSchema(t *testing.T) {
	// prepare a valid schemaResponse JSON
	validSchemas := map[string]*spec.Schema{"a.v1.K": {}}
	resp := apischema.SchemaResponse{Components: apischema.SchemasComponentsWrapper{Schemas: validSchemas}}
	validJSON, err := json.Marshal(&resp)
	assert.NoError(t, err, "failed to marshal valid response")

	tests := []struct {
		name               string
		preferredResources []*metav1.APIResourceList
		err                error
		openAPIPath        string
		openAPIErr         error
		wantErr            error
		setSchema          func(mock openapi.GroupVersion)
	}{
		{
			name:        "discovery_error",
			err:         apischema.ErrGetServerPreferred,
			openAPIPath: "/api/v1",
			openAPIErr:  nil,
			wantErr:     apischema.ErrGetServerPreferred,
			setSchema:   nil,
		},
		{
			name: "successful_resolution",
			preferredResources: []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{
							Name:       "pods",
							Kind:       "Pod",
							Namespaced: true,
						},
					},
				},
			},
			openAPIPath: "/v1",
			openAPIErr:  nil,
			wantErr:     nil,
			setSchema: func(mock openapi.GroupVersion) {
				mock.(*apischemaMocks.MockGroupVersion).
					EXPECT().
					Schema("application/json").
					Return(validJSON, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dc := apischemaMocks.NewMockDiscoveryInterface(t)
			rm := apischemaMocks.NewMockRESTMapper(t)

			// First call in resolveSchema
			dc.EXPECT().ServerPreferredResources().Return(tc.preferredResources, tc.err)

			if tc.err == nil {
				mockGV := apischemaMocks.NewMockGroupVersion(t)
				if tc.setSchema != nil {
					tc.setSchema(mockGV)
				}
				openAPIPaths := map[string]openapi.GroupVersion{
					tc.openAPIPath: mockGV,
				}
				openAPIClient := apischemaMocks.NewMockClient(t)
				openAPIClient.EXPECT().Paths().Return(openAPIPaths, tc.openAPIErr)
				dc.EXPECT().OpenAPIV3().Return(openAPIClient)
			}

			got, err := apischema.ResolveSchema(dc, rm, testlogger.New().Logger)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			assert.NoError(t, err, "unexpected error")
			assert.NotEmpty(t, got, "expected non-empty schema map")
		})
	}
}

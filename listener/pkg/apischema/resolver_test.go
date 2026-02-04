package apischema_test

import (
	"encoding/json"
	"testing"

	apischema "github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	apischemaMocks "github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

// TestResolveSchema tests the resolveSchema function. It checks if the function
// correctly resolves the schema for a given CRD and handles various error cases.
func TestResolveSchema(t *testing.T) {
	// prepare a valid schemaResponse JSON
	validSchemas := map[string]*spec.Schema{"a.v1.K": {}}
	resp := spec3.OpenAPI{Components: &spec3.Components{Schemas: validSchemas}}
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
			setSchema: func(gv openapi.GroupVersion) {
				gv.(*apischemaMocks.MockGroupVersion).
					EXPECT().
					Schema(mock.Anything).
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

			got, err := apischema.ResolveSchema(t.Context(), dc, rm)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			assert.NoError(t, err, "unexpected error")
			assert.NotEmpty(t, got, "expected non-empty schema map")
		})
	}
}

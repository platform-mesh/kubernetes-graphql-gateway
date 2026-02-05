package apischema_test

import (
	"encoding/json"
	"testing"

	apischema "github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	apischemaMocks "github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

func TestResolveSchema(t *testing.T) {
	// prepare a valid schemaResponse JSON
	validSchemas := map[string]*spec.Schema{"a.v1.K": {}}
	resp := spec3.OpenAPI{Components: &spec3.Components{Schemas: validSchemas}}
	validJSON, err := json.Marshal(&resp)
	assert.NoError(t, err, "failed to marshal valid response")

	tests := []struct {
		name       string
		openAPIErr error
		wantErr    bool
		setSchema  func(mock openapi.GroupVersion)
	}{
		{
			name:       "successful_resolution",
			openAPIErr: nil,
			wantErr:    false,
			setSchema: func(gv openapi.GroupVersion) {
				gv.(*apischemaMocks.MockGroupVersion).
					EXPECT().
					Schema(mock.Anything).
					Return(validJSON, nil)
			},
		},
		{
			name:       "openapi_path_error",
			openAPIErr: apischema.ErrGetOpenAPIPaths,
			wantErr:    true,
			setSchema:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			openAPIClient := apischemaMocks.NewMockClient(t)

			if tc.openAPIErr != nil {
				openAPIClient.EXPECT().Paths().Return(nil, tc.openAPIErr)
			} else {
				mockGV := apischemaMocks.NewMockGroupVersion(t)
				if tc.setSchema != nil {
					tc.setSchema(mockGV)
				}
				openAPIPaths := map[string]openapi.GroupVersion{
					"/v1": mockGV,
				}
				openAPIClient.EXPECT().Paths().Return(openAPIPaths, nil)
			}

			got, err := apischema.NewResolver().Resolve(t.Context(), openAPIClient)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err, "unexpected error")
			assert.NotEmpty(t, got, "expected non-empty schema")
		})
	}
}

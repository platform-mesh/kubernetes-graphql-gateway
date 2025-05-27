package apischema_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/openapi"

	"github.com/openmfp/kubernetes-graphql-gateway/listener/apischema"
	apischemaMocks "github.com/openmfp/kubernetes-graphql-gateway/listener/apischema/mocks"
	kcpMocks "github.com/openmfp/kubernetes-graphql-gateway/listener/kcp/mocks"
)

// Compile-time check that ResolverProvider implements Resolver interface
var _ apischema.Resolver = (*apischema.ResolverProvider)(nil)

// TestNewResolverNotNil checks if NewResolver() returns a non-nil *ResolverProvider
// instance. This is a runtime check to ensure that the function behaves as expected.
func TestNewResolverNotNil(t *testing.T) {
	r := apischema.NewResolver()
	assert.NotNil(t, r, "NewResolver() should return non-nil *ResolverProvider")
}

// TestResolverProvider_Resolve tests the Resolve method of the ResolverProvider struct.
func TestResolverProvider_Resolve(t *testing.T) {
	tests := []struct {
		name               string
		preferredResources []*metav1.APIResourceList
		err                error
		openAPIPaths       map[string]openapi.GroupVersion
		openAPIErr         error
		wantErr            bool
	}{
		{
			name: "discovery_error",
			err:  apischema.ErrGetServerPreferred,
			openAPIPaths: map[string]openapi.GroupVersion{
				"/api/v1": apischemaMocks.NewMockGroupVersion(t),
			},
			wantErr: true,
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
			openAPIPaths: map[string]openapi.GroupVersion{
				"/api/v1": apischemaMocks.NewMockGroupVersion(t),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := apischema.NewResolver()
			dc := kcpMocks.NewMockDiscoveryInterface(t)
			rm := kcpMocks.NewMockRESTMapper(t)

			// First call in resolveSchema
			dc.EXPECT().ServerPreferredResources().Return(tt.preferredResources, tt.err)

			// These calls are only made if ServerPreferredResources succeeds
			if tt.err == nil {
				openAPIClient := apischemaMocks.NewMockClient(t)
				openAPIClient.EXPECT().Paths().Return(tt.openAPIPaths, tt.openAPIErr)
				dc.EXPECT().OpenAPIV3().Return(openAPIClient)
			}

			got, err := resolver.Resolve(dc, rm)
			if tt.wantErr {
				assert.Error(t, err, "should return error")
			} else {
				assert.NoError(t, err, "should not return error")
				assert.NotNil(t, got, "should return non-nil result when no error expected")
			}
		})
	}
}

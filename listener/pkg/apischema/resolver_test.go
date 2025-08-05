package apischema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/openapi"

	"github.com/openmfp/golang-commons/logger/testlogger"
	apischemaMocks "github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/apischema/mocks"
)

// Compile-time check that ResolverProvider implements Resolver interface
var _ Resolver = (*ResolverProvider)(nil)

// TestNewResolverNotNil checks if NewResolver() returns a non-nil *ResolverProvider
// instance. This is a runtime check to ensure that the function behaves as expected.
func TestNewResolverNotNil(t *testing.T) {
	r := NewResolver(testlogger.New().Logger)
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
			err:  ErrGetServerPreferred,
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
			resolver := NewResolver(testlogger.New().Logger)
			dc := apischemaMocks.NewMockDiscoveryInterface(t)
			rm := apischemaMocks.NewMockRESTMapper(t)

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

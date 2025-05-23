package apischema

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/openapi"

	"github.com/openmfp/kubernetes-graphql-gateway/listener/kcp/mocks"
)

type resolverMockOpenAPIClient struct {
	paths map[string]openapi.GroupVersion
	err   error
}

type mockRESTMapper struct{}

func (m *resolverMockOpenAPIClient) Paths() (map[string]openapi.GroupVersion, error) {
	return m.paths, m.err
}

func (m *mockRESTMapper) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (m *mockRESTMapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return nil, nil
}

func (m *mockRESTMapper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{}, nil
}

func (m *mockRESTMapper) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return nil, nil
}

func (m *mockRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	return nil, nil
}

func (m *mockRESTMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	return nil, nil
}

func (m *mockRESTMapper) ResourceSingularizer(resource string) (singular string, err error) {
	return "", nil
}

// Compile-time check that ResolverProvider implements Resolver interface
var _ Resolver = (*ResolverProvider)(nil)

// TestNewResolverNotNil checks if NewResolver() returns a non-nil *ResolverProvider
// instance. This is a runtime check to ensure that the function behaves as expected.
func TestNewResolverNotNil(t *testing.T) {
	r := NewResolver()
	if r == nil {
		t.Fatal("NewResolver() returned nil, expected non-nil *ResolverProvider")
	}
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
			name: "discovery error",
			err:  ErrGetServerPreferred,
			openAPIPaths: map[string]openapi.GroupVersion{
				"/api/v1": &fakeGV{},
			},
			wantErr: true,
		},
		{
			name: "successful resolution",
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
				"/api/v1": &fakeGV{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewResolver()
			dc := mocks.NewMockDiscoveryInterface(t)

			// First call in resolveSchema
			dc.EXPECT().ServerPreferredResources().Return(tt.preferredResources, tt.err)

			// These calls are only made if ServerPreferredResources succeeds
			if tt.err == nil {
				openAPIClient := &resolverMockOpenAPIClient{
					paths: tt.openAPIPaths,
					err:   tt.openAPIErr,
				}
				dc.EXPECT().OpenAPIV3().Return(openAPIClient)
			}

			rm := &mockRESTMapper{}

			got, err := resolver.Resolve(dc, rm)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resolver.Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Error("Resolver.Resolve() returned nil result when no error expected")
			}
		})
	}
}

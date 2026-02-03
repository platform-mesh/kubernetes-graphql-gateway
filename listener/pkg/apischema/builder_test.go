package apischema_test

import (
	"errors"
	"testing"

	"github.com/platform-mesh/golang-commons/logger/testlogger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/apis"
	apischema "github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	apischemaMocks "github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

// TestGetOpenAPISchemaKey tests the getOpenAPISchemaKey function. It checks if the
// function correctly formats the GroupVersionKind into the expected schema key format.
func TestGetOpenAPISchemaKey(t *testing.T) {
	tests := []struct {
		gvk  metav1.GroupVersionKind
		want string
	}{
		{
			gvk:  metav1.GroupVersionKind{Group: "example.group", Version: "v1", Kind: "KindA"},
			want: "group.example.v1.KindA",
		},
		{
			gvk:  metav1.GroupVersionKind{Group: "io.openmfp.core", Version: "v2beta", Kind: "MyCRD"},
			want: "core.openmfp.io.v2beta.MyCRD",
		},
	}

	for _, tc := range tests {
		got := apischema.GetOpenAPISchemaKey(tc.gvk)
		assert.Equal(t, tc.want, got, "getOpenAPISchemaKey(%+v) result mismatch", tc.gvk)
	}
}

// TestNewSchemaBuilder tests the NewSchemaBuilder function. It checks if the
// SchemaBuilder is correctly initialized with the expected number of schemas
// and the expected schema key.
func TestNewSchemaBuilder(t *testing.T) {
	tests := []struct {
		name    string
		client  openapi.Client
		wantErr error
		wantLen int
		wantKey string
	}{
		{
			name: "populates_schemas",
			client: func() openapi.Client {
				schemaClient := apischemaMocks.NewMockClient(t)
				mockGV := apischemaMocks.NewMockGroupVersion(t)
				paths := map[string]openapi.GroupVersion{
					"/v1": mockGV,
				}
				schemaClient.EXPECT().Paths().Return(paths, nil)
				mockGV.EXPECT().Schema(mock.Anything).Return([]byte(`{
					"components": {
						"schemas": {
							"v1.Pod": {
								"type": "object",
								"x-kubernetes-group-version-kind": [{"group": "", "kind": "Pod", "version": "v1"}]
							}
						}
					}
				}`), nil)
				return schemaClient
			}(),
			wantErr: nil,
			wantLen: 1,
			wantKey: "v1.Pod",
		},
		{
			name: "error_on_Paths",
			client: func() openapi.Client {
				mock := apischemaMocks.NewMockClient(t)
				mock.EXPECT().Paths().Return(nil, errors.New("paths error"))
				return mock
			}(),
			wantErr: apischema.ErrGetOpenAPIPaths,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := apischema.NewSchemaBuilder(tc.client, []string{"v1"}, testlogger.New().Logger)
			if tc.wantErr != nil {
				assert.NotNil(t, b.GetError(), "expected error, got nil")
				assert.Equal(t, 0, len(b.GetSchemas()), "expected 0 schemas on error")
				return
			}
			assert.Equal(t, tc.wantLen, len(b.GetSchemas()), "schema count mismatch")
			if tc.wantKey != "" {
				_, ok := b.GetSchemas()[tc.wantKey]
				assert.True(t, ok, "schema key %s not found in builder.schemas", tc.wantKey)
			}
		})
	}
}

// TestWithApiResourceCategories tests the WithApiResourceCategories method
// for the SchemaBuilder struct. It checks if the categories are correctly added
// to the schema's extensions.
func TestWithApiResourceCategories(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		list     []*metav1.APIResourceList
		wantCats []string
	}{
		{
			name: "adds_categories",
			key:  "h.v1.P",
			list: []*metav1.APIResourceList{{
				GroupVersion: "h/v1",
				APIResources: []metav1.APIResource{{Kind: "P", Categories: []string{"A", "B"}}},
			}},
			wantCats: []string{"A", "B"},
		},
		{
			name: "no_categories",
			key:  "h.v1.P",
			list: []*metav1.APIResourceList{{
				GroupVersion: "h/v1",
				APIResources: []metav1.APIResource{{Kind: "P"}},
			}},
			wantCats: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := apischemaMocks.NewMockClient(t)
			mock.EXPECT().Paths().Return(map[string]openapi.GroupVersion{}, nil)
			b := apischema.NewSchemaBuilder(mock, nil, testlogger.New().Logger)
			b.SetSchemas(map[string]*spec.Schema{
				tc.key: {VendorExtensible: spec.VendorExtensible{Extensions: map[string]any{}}},
			})
			b.WithApiResourceCategories(tc.list)
			ext, found := b.GetSchemas()[tc.key].Extensions[apis.CategoriesExtensionKey]
			if tc.wantCats == nil {
				assert.False(t, found, "expected no categories")
				return
			}
			assert.True(t, found, "expected CategoriesExtensionKey to be set")
			cats, ok := ext.([]string)
			assert.True(t, ok, "categories should be []string")
			assert.Equal(t, len(tc.wantCats), len(cats))
			assert.Equal(t, tc.wantCats, cats, "categories mismatch")
		})
	}
}

// TestWithScope tests the WithScope method for the SchemaBuilder struct.
func TestWithScope(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "g", Version: "v1", Kind: "K"}

	// Create schema with GVK extension
	s := &spec.Schema{
		VendorExtensible: spec.VendorExtensible{
			Extensions: map[string]any{
				apis.GVKExtensionKey: []map[string]string{
					{"group": gvk.Group, "version": gvk.Version, "kind": gvk.Kind},
				},
			},
		},
	}

	mock := apischemaMocks.NewMockClient(t)
	mock.EXPECT().Paths().Return(map[string]openapi.GroupVersion{}, nil)
	b := apischema.NewSchemaBuilder(mock, nil, testlogger.New().Logger)
	b.SetSchemas(map[string]*spec.Schema{
		"g.v1.K": s,
	})

	// Create RESTMapper and mark GVK as namespaced
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{gvk.GroupVersion()})
	mapper.Add(gvk, meta.RESTScopeNamespace)

	b.WithScope(mapper)

	// Validate
	scope := b.GetSchemas()["g.v1.K"].Extensions[apis.ScopeExtensionKey]
	assert.Equal(t, apiextensionsv1.NamespaceScoped, scope, "scope value mismatch")
}

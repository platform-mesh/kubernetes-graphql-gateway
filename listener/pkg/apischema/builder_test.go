package apischema_test

import (
	"errors"
	"testing"

	"github.com/openmfp/golang-commons/logger/testlogger"
	"github.com/openmfp/kubernetes-graphql-gateway/common"
	apischema "github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/apischema"
	apischemaMocks "github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/apischema/mocks"
	"github.com/stretchr/testify/assert"
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

// TestGetCRDGroupVersionKind tests the getCRDGroupVersionKind function. It checks if the
// function correctly extracts the GroupVersionKind from the CRD spec and handles errors.
func TestGetCRDGroupVersionKind(t *testing.T) {
	tests := []struct {
		name    string
		spec    apiextensionsv1.CustomResourceDefinitionSpec
		want    *metav1.GroupVersionKind
		wantErr error
	}{
		{
			name: "has_versions",
			spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: "test.group",
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
					{Name: "v1beta1"},
					{Name: "v1"},
				},
				Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "Foo"},
			},
			want:    &metav1.GroupVersionKind{Group: "test.group", Version: "v1beta1", Kind: "Foo"},
			wantErr: nil,
		},
		{
			name: "no_versions",
			spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group:    "empty.group",
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{},
				Names:    apiextensionsv1.CustomResourceDefinitionNames{Kind: "Bar"},
			},
			want:    nil,
			wantErr: apischema.ErrCRDNoVersions,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := apischema.GetCRDGroupVersionKind(tc.spec)
			assert.Equal(t, tc.wantErr, err, "error value mismatch")
			if tc.wantErr != nil {
				return
			}
			assert.Equal(t, tc.want, got, "result value mismatch")
		})
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
				mock := apischemaMocks.NewMockClient(t)
				mockGV := apischemaMocks.NewMockGroupVersion(t)
				paths := map[string]openapi.GroupVersion{
					"/v1": mockGV,
				}
				mock.EXPECT().Paths().Return(paths, nil)
				mockGV.EXPECT().Schema("application/json").Return([]byte(`{
					"components": {
						"schemas": {
							"v1.Pod": {
								"type": "object",
								"x-kubernetes-group-version-kind": [{"group": "", "kind": "Pod", "version": "v1"}]
							}
						}
					}
				}`), nil)
				return mock
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

// TestWithCRDCategories tests the WithCRDCategories method
// for the SchemaBuilder struct. It checks if the categories are correctly added
// to the schema's extensions.
func TestWithCRDCategories(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		crd      *apiextensionsv1.CustomResourceDefinition
		wantCats []string
	}{
		{
			name: "adds_categories",
			key:  "g.v1.K",
			crd: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Group:    "g",
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1"}},
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Kind:       "K",
						Categories: []string{"cat1", "cat2"},
					},
				},
			},
			wantCats: []string{"cat1", "cat2"},
		},
		{
			name: "no_categories",
			key:  "g.v1.K",
			crd: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Group:    "g",
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1"}},
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Kind: "K",
					},
				},
			},
			wantCats: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := apischemaMocks.NewMockClient(t)
			mock.EXPECT().Paths().Return(map[string]openapi.GroupVersion{}, nil)
			b := apischema.NewSchemaBuilder(mock, nil, testlogger.New().Logger)
			b.SetSchemas(map[string]*spec.Schema{
				tc.key: {VendorExtensible: spec.VendorExtensible{Extensions: map[string]interface{}{}}},
			})
			b.WithCRDCategories(tc.crd)
			ext, found := b.GetSchemas()[tc.key].VendorExtensible.Extensions[common.CategoriesExtensionKey]
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
				tc.key: {VendorExtensible: spec.VendorExtensible{Extensions: map[string]interface{}{}}},
			})
			b.WithApiResourceCategories(tc.list)
			ext, found := b.GetSchemas()[tc.key].VendorExtensible.Extensions[common.CategoriesExtensionKey]
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
			Extensions: map[string]interface{}{
				common.GVKExtensionKey: []map[string]string{
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
	scope := b.GetSchemas()["g.v1.K"].VendorExtensible.Extensions[common.ScopeExtensionKey]
	assert.Equal(t, apiextensionsv1.NamespaceScoped, scope, "scope value mismatch")
}

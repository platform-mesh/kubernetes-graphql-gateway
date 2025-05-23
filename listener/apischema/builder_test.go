package apischema

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/openmfp/kubernetes-graphql-gateway/common"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

type fakeClient struct {
	paths map[string]openapi.GroupVersion
}

type fakeErrClient struct{}

func (f *fakeClient) Paths() (map[string]openapi.GroupVersion, error) {
	return f.paths, nil
}

func (f *fakeErrClient) Paths() (map[string]openapi.GroupVersion, error) {
	return nil, errors.New("fail Paths")
}

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
		got := getOpenAPISchemaKey(tc.gvk)
		if got != tc.want {
			t.Errorf("getOpenAPISchemaKey(%+v) = %q; want %q", tc.gvk, got, tc.want)
		}
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
			wantErr: ErrCRDNoVersions,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getCRDGroupVersionKind(tc.spec)
			if tc.wantErr != nil {
				if err != tc.wantErr {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil || *got != *tc.want {
				t.Errorf("got %+v; want %+v", got, tc.want)
			}
		})
	}
}

// TestNewSchemaBuilder tests the NewSchemaBuilder function. It checks if the
// SchemaBuilder is correctly initialized with the expected number of schemas
// and the expected schema key.
func TestNewSchemaBuilder(t *testing.T) {
	tests := []struct {
		name      string
		client    openapi.Client
		wantLen   int
		wantKey   string
		wantError bool
	}{
		{
			name: "populates_schemas",
			client: &fakeClient{paths: map[string]openapi.GroupVersion{"/X/v1": fakeGV{data: func() []byte {
				d, _ := json.Marshal(&schemaResponse{Components: schemasComponentsWrapper{Schemas: map[string]*spec.Schema{"X.v1.K": {}}}})
				return d
			}(), err: nil}}},
			wantLen: 1,
			wantKey: "X.v1.K",
		},
		{
			name:      "error_on_Paths",
			client:    &fakeErrClient{},
			wantLen:   0,
			wantError: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := NewSchemaBuilder(tc.client, []string{"X/v1"})
			if tc.wantError {
				if b.err == nil {
					t.Error("expected error, got nil")
				}
				if len(b.schemas) != 0 {
					t.Errorf("expected 0 schemas on error, got %d", len(b.schemas))
				}
				return
			}
			if len(b.schemas) != tc.wantLen {
				t.Fatalf("expected %d schema entry, got %d", tc.wantLen, len(b.schemas))
			}
			if tc.wantKey != "" {
				if _, ok := b.schemas[tc.wantKey]; !ok {
					t.Errorf("schema key %s not found in builder.schemas", tc.wantKey)
				}
			}
		})
	}
}

// TestWithCRDCategories tests the WithCRDCategories method
// for the SchemaBuilder struct. It checks if the categories are correctly added to the schema's extensions.
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
			b := &SchemaBuilder{schemas: map[string]*spec.Schema{
				tc.key: {VendorExtensible: spec.VendorExtensible{Extensions: map[string]interface{}{}}},
			}}
			b.WithCRDCategories(tc.crd)
			ext, found := b.schemas[tc.key].VendorExtensible.Extensions[common.CategoriesExtensionKey]
			if tc.wantCats == nil {
				if found {
					t.Errorf("expected no categories, but found: %#v", ext)
				}
				return
			}
			if !found {
				t.Fatal("expected CategoriesExtensionKey to be set")
			}
			cats, ok := ext.([]string)
			if !ok || len(cats) != len(tc.wantCats) || cats[0] != tc.wantCats[0] {
				t.Errorf("unexpected categories: %#v", ext)
			}
		})
	}
}

// TestWithApiResourceCategories tests the WithApiResourceCategories method
// for the SchemaBuilder struct. It checks if the categories are correctly added to the schema's extensions.
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
			b := &SchemaBuilder{schemas: map[string]*spec.Schema{
				tc.key: {VendorExtensible: spec.VendorExtensible{Extensions: map[string]interface{}{}}},
			}}
			b.WithApiResourceCategories(tc.list)
			ext, found := b.schemas[tc.key].VendorExtensible.Extensions[common.CategoriesExtensionKey]
			if tc.wantCats == nil {
				if found {
					t.Errorf("expected no categories, but found: %#v", ext)
				}
				return
			}
			if !found {
				t.Fatal("expected CategoriesExtensionKey to be set by WithApiResourceCategories")
			}
			cats, ok := ext.([]string)
			if !ok || len(cats) != len(tc.wantCats) || cats[len(tc.wantCats)-1] != tc.wantCats[len(tc.wantCats)-1] {
				t.Errorf("unexpected categories: %#v", ext)
			}
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

	b := &SchemaBuilder{
		schemas: map[string]*spec.Schema{
			"g.v1.K": s,
		},
	}

	// Create RESTMapper and mark GVK as namespaced
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{gvk.GroupVersion()})
	mapper.Add(gvk, meta.RESTScopeNamespace)

	b.WithScope(mapper)

	// Validate
	scope, ok := b.schemas["g.v1.K"].VendorExtensible.Extensions[common.ScopeExtensionKey]
	if !ok {
		t.Fatal("expected ScopeExtensionKey to be set")
	}
	if scope != apiextensionsv1.NamespaceScoped {
		t.Errorf("expected NamespaceScoped, got %v", scope)
	}
}

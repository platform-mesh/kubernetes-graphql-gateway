package apischema

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

type fakeGV struct {
	data []byte
	err  error
}

type mockOpenAPIClient struct {
	paths map[string]openapi.GroupVersion
	err   error
}

type MockCRDResolver struct {
	*CRDResolver
	preferredResources []*metav1.APIResourceList
	err                error
	openAPIClient      *mockOpenAPIClient
}

func (f fakeGV) Schema(mime string) ([]byte, error) {
	return f.data, f.err
}

func (f fakeGV) ServerRelativeURL() string {
	return ""
}

func (m *mockOpenAPIClient) Paths() (map[string]openapi.GroupVersion, error) {
	return m.paths, m.err
}

func (m *MockCRDResolver) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return m.preferredResources, m.err
}

func (m *MockCRDResolver) OpenAPIV3() openapi.Client {
	return m.openAPIClient
}

// TestGetCRDGroupKindVersions tests the getCRDGroupKindVersions function. It checks if the
// function correctly extracts the Group, Kind, and Versions from the CRD spec.
func TestGetCRDGroupKindVersions(t *testing.T) {
	tests := []struct {
		name     string
		spec     apiextensionsv1.CustomResourceDefinitionSpec
		wantG    string
		wantKind string
		wantVers []string
	}{
		{
			name:     "basic",
			spec:     apiextensionsv1.CustomResourceDefinitionSpec{Group: "test.group", Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1"}, {Name: "v2"}}, Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "MyKind"}},
			wantG:    "test.group",
			wantKind: "MyKind",
			wantVers: []string{"v1", "v2"},
		},
		{
			name:     "single_version",
			spec:     apiextensionsv1.CustomResourceDefinitionSpec{Group: "g", Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1"}}, Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "K"}},
			wantG:    "g",
			wantKind: "K",
			wantVers: []string{"v1"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gkv := getCRDGroupKindVersions(tc.spec)
			assert.Equal(t, tc.wantG, gkv.Group, "Group mismatch")
			assert.Equal(t, tc.wantKind, gkv.Kind, "Kind mismatch")
			assert.Equal(t, tc.wantVers, gkv.Versions, "Versions mismatch")
		})
	}
}

// TestIsCRDKindIncluded tests the isCRDKindIncluded function. It checks if the function correctly
// determines if a specific kind is included in the APIResourceList.
func TestIsCRDKindIncluded(t *testing.T) {
	tests := []struct {
		name    string
		gkv     *GroupKindVersions
		apiList *metav1.APIResourceList
		want    bool
	}{
		{
			name:    "kind_present",
			gkv:     &GroupKindVersions{GroupKind: &metav1.GroupKind{Group: "g", Kind: "KindA"}, Versions: []string{"v1"}},
			apiList: &metav1.APIResourceList{GroupVersion: "g/v1", APIResources: []metav1.APIResource{{Kind: "KindA"}, {Kind: "Other"}}},
			want:    true,
		},
		{
			name:    "kind_absent",
			gkv:     &GroupKindVersions{GroupKind: &metav1.GroupKind{Group: "g", Kind: "KindA"}, Versions: []string{"v1"}},
			apiList: &metav1.APIResourceList{GroupVersion: "g/v1", APIResources: []metav1.APIResource{{Kind: "Different"}}},
			want:    false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isCRDKindIncluded(tc.gkv, tc.apiList)
			assert.Equal(t, tc.want, got, "result mismatch")
		})
	}
}

// TestErrorIfCRDNotInPreferredApiGroups tests the errorIfCRDNotInPreferredApiGroups function.
// It checks if the function correctly identifies if a CRD is not in the preferred API groups.
func TestErrorIfCRDNotInPreferredApiGroups(t *testing.T) {
	gkv := &GroupKindVersions{
		GroupKind: &metav1.GroupKind{Group: "g", Kind: "K"},
		Versions:  []string{"v1", "v2"},
	}
	cases := []struct {
		name      string
		lists     []*metav1.APIResourceList
		wantErr   error
		wantGroup []string
	}{
		{
			name: "kind_found",
			lists: []*metav1.APIResourceList{
				{
					GroupVersion: "g/v2",
					APIResources: []metav1.APIResource{{Kind: "K"}},
				},
				{
					GroupVersion: "g/v3",
					APIResources: []metav1.APIResource{{Kind: "Other"}},
				},
			},
			wantErr:   nil,
			wantGroup: []string{"g/v2", "g/v3"},
		},
		{
			name:    "kind_not_found",
			lists:   []*metav1.APIResourceList{{GroupVersion: "g/v1", APIResources: []metav1.APIResource{{Kind: "X"}}}},
			wantErr: ErrGVKNotPreferred,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			groups, err := errorIfCRDNotInPreferredApiGroups(gkv, tc.lists)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err, "unexpected error")
			assert.Equal(t, tc.wantGroup, groups, "groups mismatch")
		})
	}
}

// TestGetSchemaForPath tests the getSchemaForPath function. It checks if the function
// correctly retrieves the schema for a given path and handles various error cases.
func TestGetSchemaForPath(t *testing.T) {
	// prepare a valid schemaResponse JSON
	validSchemas := map[string]*spec.Schema{"a.v1.K": {}}
	resp := schemaResponse{Components: schemasComponentsWrapper{Schemas: validSchemas}}
	validJSON, err := json.Marshal(&resp)
	if err != nil {
		t.Fatalf("failed to marshal valid response: %v", err)
	}

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
			gv:        fakeGV{},
			wantErr:   ErrInvalidPath,
		},
		{
			name:      "not_preferred",
			preferred: []string{"x/y"},
			path:      "/g/v1",
			gv:        fakeGV{},
			wantErr:   ErrNotPreferred,
		},
		{
			name:      "unmarshal_error",
			preferred: []string{"g/v1"},
			path:      "/g/v1",
			gv:        fakeGV{data: []byte("bad json"), err: nil},
			wantErr:   ErrUnmarshalSchemaForPath,
		},
		{
			name:      "success",
			preferred: []string{"g/v1"},
			path:      "/g/v1",
			gv:        fakeGV{data: validJSON, err: nil},
			wantErr:   nil,
			wantCount: len(validSchemas),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getSchemaForPath(tc.preferred, tc.path, tc.gv)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err, "unexpected error")
			assert.Equal(t, tc.wantCount, len(got), "schema count mismatch")
		})
	}
}

// TestResolveSchema tests the resolveSchema function. It checks if the function correctly
// resolves the schema for a given path and handles various error cases.
func TestResolveSchema(t *testing.T) {
	tests := []struct {
		name               string
		preferredResources []*metav1.APIResourceList
		err                error
		openAPIPaths       map[string]openapi.GroupVersion
		openAPIErr         error
		wantErr            bool
	}{
		{
			name:    "discovery_error",
			err:     ErrGetServerPreferred,
			wantErr: true,
		},
		{
			name: "successful_schema_resolution",
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
				"/api/v1": fakeGV{},
			},
			wantErr: false,
		},
		{
			name:               "empty_resources_list",
			preferredResources: []*metav1.APIResourceList{},
			openAPIPaths: map[string]openapi.GroupVersion{
				"/api/v1": fakeGV{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &MockCRDResolver{
				CRDResolver:        &CRDResolver{},
				preferredResources: tt.preferredResources,
				err:                tt.err,
				openAPIClient: &mockOpenAPIClient{
					paths: tt.openAPIPaths,
					err:   tt.openAPIErr,
				},
			}

			got, err := resolveSchema(resolver, resolver)
			if tt.wantErr {
				assert.Error(t, err, "expected an error")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.NotNil(t, got, "expected non-nil result")
			}
		})
	}
}

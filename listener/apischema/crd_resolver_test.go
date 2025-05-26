package apischema

import (
	"encoding/json"
	"errors"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/validation/spec"

	apischemaMocks "github.com/openmfp/kubernetes-graphql-gateway/listener/apischema/mocks"
	kcpMocks "github.com/openmfp/kubernetes-graphql-gateway/listener/kcp/mocks"
	"github.com/stretchr/testify/assert"
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
			if gkv.Group != tc.wantG || gkv.Kind != tc.wantKind {
				t.Errorf("GroupKind mismatch: got %v/%v, want %v/%v", gkv.Group, gkv.Kind, tc.wantG, tc.wantKind)
			}
			if len(gkv.Versions) != len(tc.wantVers) {
				t.Fatalf("Versions length: got %d, want %d", len(gkv.Versions), len(tc.wantVers))
			}
			for i, v := range tc.wantVers {
				if gkv.Versions[i] != v {
					t.Errorf("Versions[%d]: got %q, want %q", i, gkv.Versions[i], v)
				}
			}
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
			name: "kind_present",
			gkv: &GroupKindVersions{
				GroupKind: &metav1.GroupKind{
					Group: "g",
					Kind:  "KindA",
				},
				Versions: []string{"v1"},
			},
			apiList: &metav1.APIResourceList{
				GroupVersion: "g/v1",
				APIResources: []metav1.APIResource{
					{Kind: "KindA"},
					{Kind: "Other"},
				},
			},
			want: true,
		},
		{
			name: "kind_absent",
			gkv: &GroupKindVersions{
				GroupKind: &metav1.GroupKind{
					Group: "g",
					Kind:  "KindA",
				},
				Versions: []string{"v1"},
			},
			apiList: &metav1.APIResourceList{
				GroupVersion: "g/v1",
				APIResources: []metav1.APIResource{
					{Kind: "Different"},
				},
			},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isCRDKindIncluded(tc.gkv, tc.apiList)
			if got != tc.want {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
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
			name: "kind found",
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
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(groups) != len(tc.wantGroup) {
				t.Fatalf("group count: got %d, want %d", len(groups), len(tc.wantGroup))
			}
			for i := range groups {
				if groups[i] != tc.wantGroup[i] {
					t.Errorf("groups[%d]: got %q, want %q", i, groups[i], tc.wantGroup[i])
				}
			}
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
			gv:        apischemaMocks.NewMockGroupVersion(t),
			wantErr:   ErrInvalidPath,
		},
		{
			name:      "not_preferred",
			preferred: []string{"x/y"},
			path:      "/g/v1",
			gv:        apischemaMocks.NewMockGroupVersion(t),
			wantErr:   ErrNotPreferred,
		},
		{
			name:      "unmarshal error",
			preferred: []string{"g/v1"},
			path:      "/g/v1",
			gv: func() openapi.GroupVersion {
				mock := apischemaMocks.NewMockGroupVersion(t)
				mock.EXPECT().Schema("application/json").Return([]byte("bad json"), nil)
				return mock
			}(),
			wantErr: ErrUnmarshalSchemaForPath,
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
			got, err := getSchemaForPath(tc.preferred, tc.path, tc.gv)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.wantCount {
				t.Fatalf("schema count: got %d, want %d", len(got), tc.wantCount)
			}
		})
	}
}

// TestResolveSchema tests the resolveSchema function. It checks if the function
// correctly resolves the schema for a given CRD and handles various error cases.
func TestResolveSchema(t *testing.T) {
	// prepare a valid schemaResponse JSON
	validSchemas := map[string]*spec.Schema{"a.v1.K": {}}
	resp := schemaResponse{Components: schemasComponentsWrapper{Schemas: validSchemas}}
	validJSON, err := json.Marshal(&resp)
	if err != nil {
		t.Fatalf("failed to marshal valid response: %v", err)
	}

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
			err:         ErrGetServerPreferred,
			openAPIPath: "/api/v1",
			openAPIErr:  nil,
			wantErr:     ErrGetServerPreferred,
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
				mock.(*apischemaMocks.MockGroupVersion).EXPECT().Schema("application/json").Return(validJSON, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dc := kcpMocks.NewMockDiscoveryInterface(t)
			rm := kcpMocks.NewMockRESTMapper(t)

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

			got, err := resolveSchema(dc, rm)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) == 0 {
				t.Fatal("expected non-empty schema map")
			}
		})
	}
}

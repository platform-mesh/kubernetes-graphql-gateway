package resolver_test

import (
	"context"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/golang-commons/logger/testlogger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common/mocks"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestListItems(t *testing.T) {
	tests := []struct {
		name           string
		args           map[string]any
		mockSetup      func(runtimeClientMock *mocks.MockWithWatch)
		expectedResult map[string]any
		expectError    bool
	}{
		{
			name: "listItems_OK",
			args: map[string]any{
				resolver.NamespaceArg:     "test-namespace",
				resolver.LabelSelectorArg: "key=value",
				resolver.SortByArg:        "metadata.name",
			},
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					List(
						mock.Anything,
						mock.AnythingOfType("*unstructured.UnstructuredList"),
						client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(labels.Set{"key": "value"})},
						client.InNamespace("test-namespace"),
					).
					Run(func(_ context.Context, l client.ObjectList, _ ...client.ListOption) {
						ul := l.(*unstructured.UnstructuredList)
						ul.Items = []unstructured.Unstructured{
							{Object: map[string]any{"metadata": map[string]any{"name": "ns-object"}}},
						}
						ul.SetResourceVersion("12345")
						ul.SetContinue("ctok")
						var ric int64 = 12
						ul.SetRemainingItemCount(&ric)
					}).
					Return(nil)
			},
			expectedResult: map[string]any{
				"resourceVersion":    "12345",
				"continue":           "ctok",
				"remainingItemCount": int64(12),
				"items": []map[string]any{
					{"metadata": map[string]any{"name": "ns-object"}},
				},
			},
		},
		{
			name: "listItems_with_pagination_args",
			args: map[string]any{
				resolver.NamespaceArg: "test-namespace",
				resolver.SortByArg:    "metadata.name",
				resolver.LimitArg:     1,
				resolver.ContinueArg:  "abc",
			},
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					List(
						mock.Anything,
						mock.AnythingOfType("*unstructured.UnstructuredList"),
						client.InNamespace("test-namespace"),
						client.Limit(int64(1)),
						client.Continue("abc"),
					).
					Run(func(_ context.Context, l client.ObjectList, _ ...client.ListOption) {
						ul := l.(*unstructured.UnstructuredList)
						ul.Items = []unstructured.Unstructured{
							{Object: map[string]any{"metadata": map[string]any{"name": "a"}}},
						}
						ul.SetResourceVersion("200")
						ul.SetContinue("")
						var ric int64 = 0
						ul.SetRemainingItemCount(&ric)
					}).
					Return(nil)
			},
			expectedResult: map[string]any{
				"resourceVersion":    "200",
				"continue":           "",
				"remainingItemCount": int64(0),
				"items": []map[string]any{
					{"metadata": map[string]any{"name": "a"}},
				},
			},
		},
		{
			name: "listItems_ERROR",
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					List(mock.Anything, mock.AnythingOfType("*unstructured.UnstructuredList"), mock.Anything).
					Return(assert.AnError)
			},
			expectError: true,
		},
		{
			name: "invalidLabelSelector_ERROR",
			args: map[string]any{
				resolver.LabelSelectorArg: ",,",
			},
			expectedResult: nil,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtimeClientMock := &mocks.MockWithWatch{}
			if tt.mockSetup != nil {
				tt.mockSetup(runtimeClientMock)
			}

			r := resolver.New(testlogger.New().Logger, runtimeClientMock)
			result, err := r.ListItems(schema.GroupVersionKind{
				Group:   "group",
				Version: "version",
				Kind:    "kind",
			}, v1.NamespaceScoped)(graphql.ResolveParams{
				Context: context.Background(),
				Args:    tt.args,
			})

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestGetItem(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockSetup   func(runtimeClientMock *mocks.MockWithWatch)
		expectedObj map[string]any
		expectError bool
	}{
		{
			name: "getItem_OK",
			args: map[string]any{
				resolver.NameArg:      "test-object",
				resolver.NamespaceArg: "test-namespace",
			},
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					Get(
						mock.Anything,
						client.ObjectKey{Namespace: "test-namespace", Name: "test-object"},
						mock.AnythingOfType("*unstructured.Unstructured"),
					).
					Run(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
						unstructuredObj := obj.(*unstructured.Unstructured)
						unstructuredObj.Object = map[string]any{
							"metadata": map[string]any{"name": "test-object"},
						}
					}).
					Return(nil)
			},
			expectedObj: map[string]any{
				"metadata": map[string]any{"name": "test-object"},
			},
		},
		{
			name: "getItem_ERROR",
			args: map[string]any{
				resolver.NameArg:      "test-object",
				resolver.NamespaceArg: "test-namespace",
			},
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					Get(mock.Anything, client.ObjectKey{Namespace: "test-namespace", Name: "test-object"}, mock.Anything).
					Return(assert.AnError)
			},
			expectError: true,
		},
		{
			name: "missingNameArg_ERROR",
			args: map[string]any{
				resolver.NamespaceArg: "test-namespace",
			},
			expectError: true,
		},
		{
			name: "missingNamespaceArg_ERROR",
			args: map[string]any{
				resolver.NameArg: "test-object",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtimeClientMock := &mocks.MockWithWatch{}
			if tt.mockSetup != nil {
				tt.mockSetup(runtimeClientMock)
			}

			r := resolver.New(testlogger.New().Logger, runtimeClientMock)

			result, err := r.GetItem(schema.GroupVersionKind{
				Group:   "group",
				Version: "version",
				Kind:    "kind",
			}, v1.NamespaceScoped)(graphql.ResolveParams{
				Context: context.Background(),
				Args:    tt.args,
			})

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedObj, result)
			}
		})
	}
}

func TestGetItemAsYAML(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]any
		mockSetup    func(runtimeClientMock *mocks.MockWithWatch)
		expectedYAML string
		expectError  bool
	}{
		{
			name: "getItemAsYAML_OK",
			args: map[string]any{
				resolver.NameArg:      "test-object",
				resolver.NamespaceArg: "test-namespace",
			},
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					Get(
						mock.Anything,
						client.ObjectKey{Namespace: "test-namespace", Name: "test-object"},
						mock.AnythingOfType("*unstructured.Unstructured"),
					).
					Run(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
						unstructuredObj := obj.(*unstructured.Unstructured)
						unstructuredObj.Object = map[string]any{
							"metadata": map[string]any{"name": "test-object"},
						}
					}).
					Return(nil)
			},
			expectedYAML: "metadata:\n    name: test-object\n",
		},
		{
			name: "getItemAsYAML_ERROR",
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					Get(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(assert.AnError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtimeClientMock := &mocks.MockWithWatch{}
			if tt.mockSetup != nil {
				tt.mockSetup(runtimeClientMock)
			}

			r := resolver.New(testlogger.New().Logger, runtimeClientMock)

			result, err := r.GetItemAsYAML(schema.GroupVersionKind{
				Group:   "group",
				Version: "version",
				Kind:    "kind",
			}, v1.NamespaceScoped)(graphql.ResolveParams{
				Context: context.Background(),
				Args:    tt.args,
			})

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedYAML, result)
			}
		})
	}
}

func TestCreateItem(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockSetup   func(runtimeClientMock *mocks.MockWithWatch)
		expectedObj map[string]any
		expectError bool
	}{
		{
			name: "create_item_OK",
			args: map[string]any{
				resolver.NameArg:      "test-object",
				resolver.NamespaceArg: "test-namespace",
				"object": map[string]any{
					"metadata": map[string]any{
						"name": "test-object",
					},
				},
			},
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					Create(
						mock.Anything,
						mock.AnythingOfType("*unstructured.Unstructured"),
						mock.MatchedBy(func(opts client.CreateOption) bool {
							return true
						}),
					).
					Return(nil)
			},
			expectedObj: map[string]any{
				"apiVersion": "group/version",
				"kind":       "kind",
				"metadata": map[string]any{
					"name":      "test-object",
					"namespace": "test-namespace",
				},
			},
		},
		{
			name: "create_item_ERROR",
			args: map[string]any{
				resolver.NameArg:      "test-object",
				resolver.NamespaceArg: "test-namespace",
				"object": map[string]any{
					"metadata": map[string]any{
						"name": "test-object",
					},
				},
			},
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					Create(
						mock.Anything,
						mock.AnythingOfType("*unstructured.Unstructured"),
						mock.MatchedBy(func(opts client.CreateOption) bool {
							return true
						}),
					).
					Return(assert.AnError)
			},
			expectError: true,
		},
		{
			name: "missing_metadata_name_ERROR",
			args: map[string]any{
				resolver.NamespaceArg: "test-namespace",
				"object":              map[string]any{},
			},
			expectError: true,
		},
		{
			name: "create_item_with_dry_run_OK",
			args: map[string]any{
				resolver.NameArg:      "test-object",
				resolver.NamespaceArg: "test-namespace",
				resolver.DryRunArg:    true,
				"object": map[string]any{
					"metadata": map[string]any{
						"name": "test-object",
					},
				},
			},
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					Create(
						mock.Anything,
						mock.AnythingOfType("*unstructured.Unstructured"),
						mock.MatchedBy(func(opts client.CreateOption) bool {
							return true
						}),
					).
					Return(nil)
			},
			expectedObj: map[string]any{
				"apiVersion": "group/version",
				"kind":       "kind",
				"metadata": map[string]any{
					"name":      "test-object",
					"namespace": "test-namespace",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtimeClientMock := &mocks.MockWithWatch{}
			if tt.mockSetup != nil {
				tt.mockSetup(runtimeClientMock)
			}

			r := resolver.New(testlogger.New().Logger, runtimeClientMock)

			result, err := r.CreateItem(schema.GroupVersionKind{
				Group:   "group",
				Version: "version",
				Kind:    "kind",
			}, v1.NamespaceScoped)(graphql.ResolveParams{
				Context: context.Background(),
				Args:    tt.args,
			})

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedObj, result)
			}
		})
	}
}

func TestUpdateItem(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockSetup   func(runtimeClientMock *mocks.MockWithWatch)
		expectedObj map[string]any
		expectError bool
	}{
		{
			name: "update_item_OK",
			args: map[string]any{
				resolver.NameArg:      "test-object",
				resolver.NamespaceArg: "test-namespace",
				"object": map[string]any{
					"metadata": map[string]any{
						"name": "test-object",
					},
				},
			},
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					Get(
						mock.Anything,
						client.ObjectKey{Namespace: "test-namespace", Name: "test-object"},
						mock.AnythingOfType("*unstructured.Unstructured"),
					).
					Run(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
						unstructuredObj := obj.(*unstructured.Unstructured)
						unstructuredObj.Object = map[string]any{
							"metadata": map[string]any{
								"name": "test-object",
							},
						}
					}).
					Return(nil)

				runtimeClientMock.EXPECT().
					Patch(mock.Anything,
						mock.AnythingOfType("*unstructured.Unstructured"),
						mock.Anything,
						mock.MatchedBy(func(opts client.PatchOption) bool {
							return true
						}),
					).
					Return(nil)
			},
			expectedObj: map[string]any{
				"metadata": map[string]any{
					"name": "test-object",
				},
			},
		},
		{
			name: "missing_metadata_name_ERROR",
			args: map[string]any{
				resolver.NamespaceArg: "test-namespace",
				"object":              map[string]any{},
			},
			expectError: true,
		},
		{
			name: "get_existing_object_ERROR",
			args: map[string]any{
				resolver.NameArg:      "test-object",
				resolver.NamespaceArg: "test-namespace",
				"object": map[string]any{
					"metadata": map[string]any{
						"name": "test-object",
					},
				},
			},
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					Get(mock.Anything, client.ObjectKey{Namespace: "test-namespace", Name: "test-object"}, mock.Anything).
					Return(assert.AnError)
			},
			expectError: true,
		},
		{
			name: "patch_object_ERROR",
			args: map[string]any{
				resolver.NameArg:      "test-object",
				resolver.NamespaceArg: "test-namespace",
				"object": map[string]any{
					"metadata": map[string]any{
						"name": "test-object",
					},
				},
			},
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					Get(mock.Anything, client.ObjectKey{Namespace: "test-namespace", Name: "test-object"}, mock.Anything).
					Return(nil)

				runtimeClientMock.EXPECT().
					Patch(mock.Anything,
						mock.AnythingOfType("*unstructured.Unstructured"),
						mock.Anything,
						mock.MatchedBy(func(opts client.PatchOption) bool {
							return true
						}),
					).
					Return(assert.AnError)
			},
			expectError: true,
		},
		{
			name: "update_item_with_dry_run_OK",
			args: map[string]any{
				resolver.NameArg:      "test-object",
				resolver.NamespaceArg: "test-namespace",
				resolver.DryRunArg:    true,
				"object": map[string]any{
					"metadata": map[string]any{
						"name": "test-object",
					},
				},
			},
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					Get(
						mock.Anything,
						client.ObjectKey{Namespace: "test-namespace", Name: "test-object"},
						mock.AnythingOfType("*unstructured.Unstructured"),
					).
					Run(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
						unstructuredObj := obj.(*unstructured.Unstructured)
						unstructuredObj.Object = map[string]any{
							"metadata": map[string]any{
								"name": "test-object",
							},
						}
					}).
					Return(nil)

				runtimeClientMock.EXPECT().
					Patch(
						mock.Anything,
						mock.AnythingOfType("*unstructured.Unstructured"),
						mock.Anything,
						mock.MatchedBy(func(opts client.PatchOption) bool {
							return true
						}),
					).
					Return(nil)
			},
			expectedObj: map[string]any{
				"metadata": map[string]any{
					"name": "test-object",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtimeClientMock := &mocks.MockWithWatch{}
			if tt.mockSetup != nil {
				tt.mockSetup(runtimeClientMock)
			}

			r := resolver.New(testlogger.New().Logger, runtimeClientMock)

			result, err := r.UpdateItem(schema.GroupVersionKind{
				Group:   "group",
				Version: "version",
				Kind:    "kind",
			}, v1.NamespaceScoped)(graphql.ResolveParams{
				Context: context.Background(),
				Args:    tt.args,
			})

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedObj, result)
			}
		})
	}
}

func TestDeleteItem(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockSetup   func(runtimeClientMock *mocks.MockWithWatch)
		expectError bool
	}{
		{
			name: "delete_item_OK",
			args: map[string]any{
				resolver.NameArg:      "test-object",
				resolver.NamespaceArg: "test-namespace",
			},
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					Delete(
						mock.Anything,
						mock.AnythingOfType("*unstructured.Unstructured"),
						mock.AnythingOfType("*client.DeleteOptions"),
					).
					Return(nil)
			},
		},
		{
			name: "missing_name_argument_ERROR",
			args: map[string]any{
				resolver.NamespaceArg: "test-namespace",
			},
			expectError: true,
		},
		{
			name: "missing_namespace_argument_ERROR",
			args: map[string]any{
				resolver.NameArg: "test-object",
			},
			expectError: true,
		},
		{
			name: "delete_object_ERROR",
			args: map[string]any{
				resolver.NameArg:      "test-object",
				resolver.NamespaceArg: "test-namespace",
			},
			mockSetup: func(runtimeClientMock *mocks.MockWithWatch) {
				runtimeClientMock.EXPECT().
					Delete(
						mock.Anything,
						mock.AnythingOfType("*unstructured.Unstructured"),
						mock.AnythingOfType("*client.DeleteOptions"),
					).
					Return(assert.AnError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtimeClientMock := &mocks.MockWithWatch{}
			if tt.mockSetup != nil {
				tt.mockSetup(runtimeClientMock)
			}

			r := resolver.New(testlogger.New().Logger, runtimeClientMock)

			result, err := r.DeleteItem(schema.GroupVersionKind{
				Group:   "group",
				Version: "version",
				Kind:    "kind",
			}, v1.NamespaceScoped)(graphql.ResolveParams{
				Context: context.Background(),
				Args:    tt.args,
			})

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, true, result)
			}
		})
	}
}

func TestSanitizeGroupName(t *testing.T) {
	r := &resolver.Service{}
	r.SetGroupNames(make(map[string]string))

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty_string", "", ""},
		{"valid_group_name", "validName", "validName"},
		{"hyphen_to_underscore", "group-name", "group_name"},
		{"special_char_to_underscore", "group@name", "group_name"},
		{"invalid_start_with_prepend", "!invalidStart", "_invalidStart"},
		{"leading_underscore", "_leadingUnderscore", "_leadingUnderscore"},
		{"start_with_number", "123startWithNumber", "_123startWithNumber"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.SanitizeGroupName(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.input, r.GetGroupName(result), "The original group name should be stored correctly")
		})
	}
}

func TestGetOriginalGroupName(t *testing.T) {
	r := &resolver.Service{}
	r.SetGroupNames(map[string]string{
		"group1": "originalGroup1",
		"group2": "originalGroup2",
	})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"existing_group", "group1", "originalGroup1"},
		{"non_existing_group", "group3", "group3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.GetOriginalGroupName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCompareUnstructured(t *testing.T) {
	tests := []struct {
		name     string
		a        map[string]any
		b        map[string]any
		expected int
	}{
		{
			name:     "equal_strings",
			a:        map[string]any{"key": "abc"},
			b:        map[string]any{"key": "abc"},
			expected: 0,
		},
		{
			name:     "different_strings",
			a:        map[string]any{"key": "abc"},
			b:        map[string]any{"key": "xyz"},
			expected: -1,
		},
		{
			name:     "equal_int64",
			a:        map[string]any{"key": int64(42)},
			b:        map[string]any{"key": int64(42)},
			expected: 0,
		},
		{
			name:     "different_int64",
			a:        map[string]any{"key": int64(10)},
			b:        map[string]any{"key": int64(20)},
			expected: -1,
		},
		{
			name:     "equal_int32",
			a:        map[string]any{"key": int32(42)},
			b:        map[string]any{"key": int32(42)},
			expected: 0,
		},
		{
			name:     "different_int32",
			a:        map[string]any{"key": int32(10)},
			b:        map[string]any{"key": int32(20)},
			expected: -1,
		},
		{
			name:     "int32_vs_int64",
			a:        map[string]any{"key": int32(10)},
			b:        map[string]any{"key": int64(20)},
			expected: -1,
		},
		{
			name:     "equal_float64",
			a:        map[string]any{"key": float64(3.14)},
			b:        map[string]any{"key": float64(3.14)},
			expected: 0,
		},
		{
			name:     "different_float64",
			a:        map[string]any{"key": float64(1.5)},
			b:        map[string]any{"key": float64(2.5)},
			expected: -1,
		},
		{
			name:     "equal_float32",
			a:        map[string]any{"key": float32(3.14)},
			b:        map[string]any{"key": float32(3.14)},
			expected: 0,
		},
		{
			name:     "different_float32",
			a:        map[string]any{"key": float32(1.5)},
			b:        map[string]any{"key": float32(2.5)},
			expected: -1,
		},
		{
			name:     "float32_vs_float64",
			a:        map[string]any{"key": float32(1.5)},
			b:        map[string]any{"key": float64(2.5)},
			expected: -1,
		},
		{
			name:     "equal_bool",
			a:        map[string]any{"key": true},
			b:        map[string]any{"key": true},
			expected: 0,
		},
		{
			name:     "different_bool",
			a:        map[string]any{"key": true},
			b:        map[string]any{"key": false},
			expected: -1,
		},
		{
			name:     "missing_field",
			a:        map[string]any{},
			b:        map[string]any{"key": "abc"},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := unstructured.Unstructured{Object: tt.a}
			b := unstructured.Unstructured{Object: tt.b}
			got := resolver.CompareUnstructured(a, b, "key")
			if got != tt.expected {
				t.Errorf("compareUnstructured() = %d, want %d", got, tt.expected)
			}
		})
	}
}

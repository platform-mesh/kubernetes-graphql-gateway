package schema

import (
	"testing"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis"
	"github.com/stretchr/testify/assert"

	"k8s.io/kube-openapi/pkg/validation/spec"
)

func TestHighestSemverVersion(t *testing.T) {
	tests := []struct {
		name        string
		currentKind string
		otherKinds  map[string]*spec.Schema
		definitions map[string]*spec.Schema
		expected    string
	}{
		{
			name:        "v1 is highest among v1alpha1, v1beta1, v1",
			currentKind: "io.example.v1.MyResource",
			otherKinds: map[string]*spec.Schema{
				"io.example.v1alpha1.MyResource": {},
				"io.example.v1beta1.MyResource":  {},
			},
			definitions: map[string]*spec.Schema{
				"io.example.v1.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
				"io.example.v1alpha1.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1alpha1",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
				"io.example.v1beta1.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1beta1",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
			},
			expected: "io.example.v1.MyResource",
		},
		{
			name:        "v2 is highest among v1 and v2",
			currentKind: "io.example.v1.MyResource",
			otherKinds: map[string]*spec.Schema{
				"io.example.v2.MyResource": {},
			},
			definitions: map[string]*spec.Schema{
				"io.example.v1.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
				"io.example.v2.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v2",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
			},
			expected: "io.example.v2.MyResource",
		},
		{
			name:        "v1beta2 is highest among v1beta1 and v1beta2",
			currentKind: "io.example.v1beta1.MyResource",
			otherKinds: map[string]*spec.Schema{
				"io.example.v1beta2.MyResource": {},
			},
			definitions: map[string]*spec.Schema{
				"io.example.v1beta1.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1beta1",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
				"io.example.v1beta2.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1beta2",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
			},
			expected: "io.example.v1beta2.MyResource",
		},
		{
			name:        "current version is highest when no other versions",
			currentKind: "io.example.v1.MyResource",
			otherKinds:  map[string]*spec.Schema{},
			definitions: map[string]*spec.Schema{
				"io.example.v1.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
			},
			expected: "io.example.v1.MyResource",
		},
		{
			name:        "v1beta1 is highest among alphas",
			currentKind: "io.example.v1alpha1.MyResource",
			otherKinds: map[string]*spec.Schema{
				"io.example.v1alpha2.MyResource": {},
				"io.example.v1beta1.MyResource":  {},
			},
			definitions: map[string]*spec.Schema{
				"io.example.v1alpha1.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1alpha1",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
				"io.example.v1alpha2.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1alpha2",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
				"io.example.v1beta1.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1beta1",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
			},
			expected: "io.example.v1beta1.MyResource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := highestSemverVersion(tt.currentKind, tt.otherKinds, tt.definitions)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasAnotherVersion(t *testing.T) {
	tests := []struct {
		name               string
		currentKind        string
		allKinds           map[string]*spec.Schema
		definitions        map[string]*spec.Schema
		expectedHasOther   bool
		expectedOtherCount int
		expectedOtherKeys  []string
	}{
		{
			name:        "has other versions - v1, v1alpha1, v1beta1",
			currentKind: "io.example.v1.MyResource",
			allKinds: map[string]*spec.Schema{
				"io.example.v1.MyResource":       {},
				"io.example.v1alpha1.MyResource": {},
				"io.example.v1beta1.MyResource":  {},
			},
			definitions: map[string]*spec.Schema{
				"io.example.v1.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
				"io.example.v1alpha1.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1alpha1",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
				"io.example.v1beta1.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1beta1",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
			},
			expectedHasOther:   true,
			expectedOtherCount: 2,
			expectedOtherKeys:  []string{"io.example.v1alpha1.MyResource", "io.example.v1beta1.MyResource"},
		},
		{
			name:        "no other versions - single resource",
			currentKind: "io.example.v1.MyResource",
			allKinds: map[string]*spec.Schema{
				"io.example.v1.MyResource": {},
			},
			definitions: map[string]*spec.Schema{
				"io.example.v1.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
			},
			expectedHasOther:   false,
			expectedOtherCount: 0,
			expectedOtherKeys:  []string{},
		},
		{
			name:        "different kind with similar name should not match",
			currentKind: "io.example.v1.MyResource",
			allKinds: map[string]*spec.Schema{
				"io.example.v1.MyResource":      {},
				"io.example.v1.OtherMyResource": {},
			},
			definitions: map[string]*spec.Schema{
				"io.example.v1.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
				"io.example.v1.OtherMyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1",
									"kind":    "OtherMyResource",
								},
							},
						},
					},
				},
			},
			expectedHasOther:   false,
			expectedOtherCount: 0,
			expectedOtherKeys:  []string{},
		},
		{
			name:        "different group with same kind should not match",
			currentKind: "io.example.v1.Pod",
			allKinds: map[string]*spec.Schema{
				"io.example.v1.Pod": {},
				"io.other.v1.Pod":   {},
			},
			definitions: map[string]*spec.Schema{
				"io.example.v1.Pod": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1",
									"kind":    "Pod",
								},
							},
						},
					},
				},
				"io.other.v1.Pod": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.other",
									"version": "v1",
									"kind":    "Pod",
								},
							},
						},
					},
				},
			},
			expectedHasOther:   false,
			expectedOtherCount: 0,
			expectedOtherKeys:  []string{},
		},
		{
			name:        "same version should not be returned",
			currentKind: "io.example.v1.MyResource",
			allKinds: map[string]*spec.Schema{
				"io.example.v1.MyResource": {},
				"io.example.v2.MyResource": {},
				"io.example.v1.OtherKind":  {},
			},
			definitions: map[string]*spec.Schema{
				"io.example.v1.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
				"io.example.v2.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v2",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
				"io.example.v1.OtherKind": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1",
									"kind":    "OtherKind",
								},
							},
						},
					},
				},
			},
			expectedHasOther:   true,
			expectedOtherCount: 1,
			expectedOtherKeys:  []string{"io.example.v2.MyResource"},
		},
		{
			name:        "handle missing definition gracefully",
			currentKind: "io.example.v1.MyResource",
			allKinds: map[string]*spec.Schema{
				"io.example.v1.MyResource":  {},
				"io.example.v2.MissingDef":  {},
				"io.example.v2.MyResource2": {},
			},
			definitions: map[string]*spec.Schema{
				"io.example.v1.MyResource": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v1",
									"kind":    "MyResource",
								},
							},
						},
					},
				},
				// v2.MissingDef intentionally not in definitions
				"io.example.v2.MyResource2": {
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]any{
							apis.GVKExtensionKey: []any{
								map[string]any{
									"group":   "io.example",
									"version": "v2",
									"kind":    "MyResource2", // Different kind, shouldn't match
								},
							},
						},
					},
				},
			},
			expectedHasOther:   false,
			expectedOtherCount: 0,
			expectedOtherKeys:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasOther, otherVersions := hasAnotherVersion(tt.currentKind, tt.allKinds, tt.definitions)

			assert.Equal(t, tt.expectedHasOther, hasOther, "hasOther mismatch")
			assert.Equal(t, tt.expectedOtherCount, len(otherVersions), "otherVersions count mismatch")

			// Check that all expected keys are present
			for _, expectedKey := range tt.expectedOtherKeys {
				assert.Contains(t, otherVersions, expectedKey, "Expected key %s not found in otherVersions", expectedKey)
			}

			// Check that no unexpected keys are present
			for actualKey := range otherVersions {
				found := false
				for _, expectedKey := range tt.expectedOtherKeys {
					if actualKey == expectedKey {
						found = true
						break
					}
				}
				assert.True(t, found, "Unexpected key %s found in otherVersions", actualKey)
			}
		})
	}
}

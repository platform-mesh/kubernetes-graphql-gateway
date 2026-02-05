package apischema_test

import (
	"testing"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/stretchr/testify/assert"

	"k8s.io/kube-openapi/pkg/validation/spec"
)

func TestExtractGVK(t *testing.T) {
	tests := []struct {
		name    string
		schema  *spec.Schema
		wantGVK *apischema.GroupVersionKind
		wantErr bool
	}{
		{
			name: "valid GVK",
			schema: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: map[string]any{
						apis.GVKExtensionKey: []any{
							map[string]any{
								"group":   "apps",
								"version": "v1",
								"kind":    "Deployment",
							},
						},
					},
				},
			},
			wantGVK: &apischema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			wantErr: false,
		},
		{
			name: "core group (empty)",
			schema: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: map[string]any{
						apis.GVKExtensionKey: []any{
							map[string]any{
								"group":   "",
								"version": "v1",
								"kind":    "Pod",
							},
						},
					},
				},
			},
			wantGVK: &apischema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			wantErr: false,
		},
		{
			name:    "no extensions",
			schema:  &spec.Schema{},
			wantGVK: nil,
			wantErr: false,
		},
		{
			name: "no GVK extension",
			schema: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: map[string]any{
						"x-other": "value",
					},
				},
			},
			wantGVK: nil,
			wantErr: false,
		},
		{
			name: "multiple GVKs (skipped)",
			schema: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: map[string]any{
						apis.GVKExtensionKey: []any{
							map[string]any{"group": "a", "version": "v1", "kind": "A"},
							map[string]any{"group": "b", "version": "v1", "kind": "B"},
						},
					},
				},
			},
			wantGVK: nil, // Schemas with multiple GVKs are skipped
			wantErr: false,
		},
		{
			name: "empty GVK list",
			schema: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: map[string]any{
						apis.GVKExtensionKey: []any{},
					},
				},
			},
			wantGVK: nil,
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gvk, err := apischema.ExtractGVK(tc.schema)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.wantGVK, gvk)
		})
	}
}

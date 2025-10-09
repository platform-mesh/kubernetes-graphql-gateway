package schema_test

import (
	"testing"

	"github.com/go-openapi/spec"
	"github.com/graphql-go/graphql"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/platform-mesh/golang-commons/logger/testlogger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
	gatewaySchema "github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema"
)

func TestConvertSwaggerTypeToGraphQL_WithNilInCache(t *testing.T) {
	log := testlogger.New().Logger
	mockResolver := &mockResolverProvider{}

	tests := []struct {
		name               string
		definitions        spec.Definitions
		setupCache         func(*gatewaySchema.Gateway)
		expectedNoPanic    bool
		expectedReturnType bool
	}{
		{
			name: "handles_nil_in_cache_for_recursive_ref",
			definitions: spec.Definitions{
				"io.test.v1.RecursiveType": spec.Schema{
					SchemaProps: spec.SchemaProps{
						Type: []string{"object"},
						Properties: map[string]spec.Schema{
							"name": {
								SchemaProps: spec.SchemaProps{
									Type: []string{"string"},
								},
							},
							"parent": {
								SchemaProps: spec.SchemaProps{
									Ref: spec.MustCreateRef("#/definitions/io.test.v1.RecursiveType"),
								},
							},
						},
					},
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]interface{}{
							"x-kubernetes-group-version-kind": []interface{}{
								map[string]interface{}{
									"group":   "test",
									"version": "v1",
									"kind":    "RecursiveType",
								},
							},
						},
					},
				},
			},
			expectedNoPanic:    true,
			expectedReturnType: true,
		},
		{
			name: "handles_nested_object_with_nil_in_cache",
			definitions: spec.Definitions{
				"io.test.v1.NestedType": spec.Schema{
					SchemaProps: spec.SchemaProps{
						Type: []string{"object"},
						Properties: map[string]spec.Schema{
							"spec": {
								SchemaProps: spec.SchemaProps{
									Type: []string{"object"},
									Properties: map[string]spec.Schema{
										"nested": {
											SchemaProps: spec.SchemaProps{
												Type: []string{"object"},
												Properties: map[string]spec.Schema{
													"field": {
														SchemaProps: spec.SchemaProps{
															Type: []string{"string"},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					VendorExtensible: spec.VendorExtensible{
						Extensions: map[string]interface{}{
							"x-kubernetes-group-version-kind": []interface{}{
								map[string]interface{}{
									"group":   "test",
									"version": "v1",
									"kind":    "NestedType",
								},
							},
						},
					},
				},
			},
			expectedNoPanic:    true,
			expectedReturnType: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if tt.expectedNoPanic {
						t.Errorf("Test panicked when it shouldn't: %v", r)
					}
				}
			}()

			gateway, err := gatewaySchema.New(log, tt.definitions, mockResolver)
			if err != nil && tt.expectedNoPanic {
				t.Errorf("Schema creation failed: %v", err)
				return
			}

			if gateway != nil && tt.expectedReturnType {
				schemaObj := gateway.GetSchema()
				if schemaObj == nil {
					t.Error("Expected schema to be created but got nil")
				}
			}
		})
	}
}

func TestHandleObjectFieldSpecType_WithNilInCache(t *testing.T) {
	log := testlogger.New().Logger
	mockResolver := &mockResolverProvider{}

	definitions := spec.Definitions{
		"io.test.v1.SelfReferencing": spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"object"},
							Properties: map[string]spec.Schema{
								"name": {
									SchemaProps: spec.SchemaProps{
										Type: []string{"string"},
									},
								},
								"labels": {
									SchemaProps: spec.SchemaProps{
										Type: []string{"object"},
										AdditionalProperties: &spec.SchemaOrBool{
											Schema: &spec.Schema{
												SchemaProps: spec.SchemaProps{
													Type: []string{"string"},
												},
											},
										},
									},
								},
							},
						},
					},
					"spec": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"object"},
							Properties: map[string]spec.Schema{
								"template": {
									SchemaProps: spec.SchemaProps{
										Ref: spec.MustCreateRef("#/definitions/io.test.v1.SelfReferencing"),
									},
								},
							},
						},
					},
				},
			},
			VendorExtensible: spec.VendorExtensible{
				Extensions: map[string]interface{}{
					"x-kubernetes-group-version-kind": []interface{}{
						map[string]interface{}{
							"group":   "test",
							"version": "v1",
							"kind":    "SelfReferencing",
						},
					},
				},
			},
		},
	}

	t.Run("no_panic_with_self_referencing_object", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Test panicked: %v", r)
			}
		}()

		gateway, err := gatewaySchema.New(log, definitions, mockResolver)
		if err != nil {
			t.Errorf("Schema creation failed: %v", err)
			return
		}

		if gateway == nil {
			t.Error("Expected gateway to be created but got nil")
			return
		}

		schemaObj := gateway.GetSchema()
		if schemaObj == nil {
			t.Error("Expected schema to be created but got nil")
		}
	})
}

func TestFindRelationTarget_WithNilInCache(t *testing.T) {
	log := testlogger.New().Logger
	mockResolver := &mockResolverProvider{}

	definitions := spec.Definitions{
		"io.test.v1.Parent": spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"childRef": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"object"},
							Properties: map[string]spec.Schema{
								"name": {
									SchemaProps: spec.SchemaProps{
										Type: []string{"string"},
									},
								},
							},
						},
					},
				},
			},
			VendorExtensible: spec.VendorExtensible{
				Extensions: map[string]interface{}{
					"x-kubernetes-group-version-kind": []interface{}{
						map[string]interface{}{
							"group":   "test",
							"version": "v1",
							"kind":    "Parent",
						},
					},
					"x-kubernetes-resource-scope": "Namespaced",
				},
			},
		},
		"io.test.v1.Child": spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
				Properties: map[string]spec.Schema{
					"parentRef": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"object"},
							Properties: map[string]spec.Schema{
								"name": {
									SchemaProps: spec.SchemaProps{
										Type: []string{"string"},
									},
								},
							},
						},
					},
				},
			},
			VendorExtensible: spec.VendorExtensible{
				Extensions: map[string]interface{}{
					"x-kubernetes-group-version-kind": []interface{}{
						map[string]interface{}{
							"group":   "test",
							"version": "v1",
							"kind":    "Child",
						},
					},
					"x-kubernetes-resource-scope": "Namespaced",
				},
			},
		},
	}

	t.Run("no_panic_with_cross_references", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Test panicked: %v", r)
			}
		}()

		gateway, err := gatewaySchema.New(log, definitions, mockResolver)
		if err != nil {
			t.Errorf("Schema creation failed: %v", err)
			return
		}

		if gateway == nil {
			t.Error("Expected gateway to be created but got nil")
			return
		}

		schemaObj := gateway.GetSchema()
		if schemaObj == nil {
			t.Error("Expected schema to be created but got nil")
		}
	})
}

type mockResolverProvider struct{}

func (m *mockResolverProvider) CommonResolver() graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		return nil, nil
	}
}

func (m *mockResolverProvider) ListItems(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		return []interface{}{}, nil
	}
}

func (m *mockResolverProvider) GetItem(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		return map[string]interface{}{}, nil
	}
}

func (m *mockResolverProvider) GetItemAsYAML(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		return "", nil
	}
}

func (m *mockResolverProvider) CreateItem(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		return map[string]interface{}{}, nil
	}
}

func (m *mockResolverProvider) UpdateItem(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		return map[string]interface{}{}, nil
	}
}

func (m *mockResolverProvider) DeleteItem(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		return true, nil
	}
}

func (m *mockResolverProvider) SubscribeItem(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		return nil, nil
	}
}

func (m *mockResolverProvider) SubscribeItems(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		return nil, nil
	}
}

func (m *mockResolverProvider) RelationResolver(baseName string, gvk schema.GroupVersionKind) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		return map[string]interface{}{}, nil
	}
}

func (m *mockResolverProvider) TypeByCategory(typeByCategory map[string][]resolver.TypeByCategory) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		return []interface{}{}, nil
	}
}

func (m *mockResolverProvider) SanitizeGroupName(group string) string {
	return group
}

var _ resolver.Provider = (*mockResolverProvider)(nil)

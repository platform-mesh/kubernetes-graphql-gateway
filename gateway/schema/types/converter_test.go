package types_test

import (
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema/types"

	"k8s.io/kube-openapi/pkg/validation/spec"
)

// TestConvert_TypelessFieldUsesJSONScalar verifies that a schema field with no
// explicit type (as produced by apiextensionsv1.JSON / runtime.RawExtension with
// x-kubernetes-preserve-unknown-fields) is mapped to JSONStringScalar instead of
// graphql.String, so values are serialized via json.Marshal rather than fmt.Sprintf.
// Regression test for https://github.com/platform-mesh/kubernetes-graphql-gateway/issues/148
func TestConvert_TypelessFieldUsesJSONScalar(t *testing.T) {
	converter := types.NewConverter(types.NewRegistry())

	schema := &spec.Schema{
		SchemaProps: spec.SchemaProps{
			Properties: map[string]spec.Schema{
				"config": {},
			},
		},
	}

	fields, inputFields, err := converter.ConvertFields(schema, map[string]*spec.Schema{}, "TestType")
	if err != nil {
		t.Fatalf("ConvertFields() error = %v", err)
	}

	if got := fields["config"].Type.Name(); got != "JSONString" {
		t.Errorf("output type = %q, want %q", got, "JSONString")
	}
	if got := inputFields["config"].Type.Name(); got != "JSONString" {
		t.Errorf("input type = %q, want %q", got, "JSONString")
	}
}

// TestConvert_NestedTypeNameCollision verifies that a CRD whose nested field
// path would produce a name matching a built-in Kind (e.g., Component + status
// → ComponentStatus) does not collide when the typePrefix is group+version
// qualified. Regression test for https://github.com/platform-mesh/kubernetes-graphql-gateway/issues/222
func TestConvert_NestedTypeNameCollision(t *testing.T) {
	registry := types.NewRegistry()
	converter := types.NewConverter(registry)

	builtinType := graphql.NewObject(graphql.ObjectConfig{
		Name:   "V1ComponentStatus",
		Fields: graphql.Fields{"name": &graphql.Field{Type: graphql.String}},
	})
	builtinInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name:   "V1ComponentStatusInput",
		Fields: graphql.InputObjectConfigFieldMap{"name": &graphql.InputObjectFieldConfig{Type: graphql.String}},
	})
	registry.Register("V1ComponentStatus", builtinType, builtinInput)

	schema := &spec.Schema{
		SchemaProps: spec.SchemaProps{
			Properties: map[string]spec.Schema{
				"status": {
					SchemaProps: spec.SchemaProps{
						Type: []string{"object"},
						Properties: map[string]spec.Schema{
							"ready": {SchemaProps: spec.SchemaProps{Type: []string{"boolean"}}},
						},
					},
				},
			},
		},
	}

	fields, inputFields, err := converter.ConvertFields(schema, map[string]*spec.Schema{}, "CustomIoV1Component")
	if err != nil {
		t.Fatalf("ConvertFields() error = %v", err)
	}

	statusField := fields["status"]
	if statusField == nil {
		t.Fatal("expected 'status' field to exist")
	}
	if got := statusField.Type.Name(); got != "CustomIoV1ComponentStatus" {
		t.Errorf("nested output type = %q, want %q", got, "CustomIoV1ComponentStatus")
	}

	statusInput := inputFields["status"]
	if statusInput == nil {
		t.Fatal("expected 'status' input field to exist")
	}
	if got := statusInput.Type.Name(); got != "CustomIoV1ComponentStatusInput" {
		t.Errorf("nested input type = %q, want %q", got, "CustomIoV1ComponentStatusInput")
	}
}

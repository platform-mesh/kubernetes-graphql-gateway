package types_test

import (
	"testing"

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

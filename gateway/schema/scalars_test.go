package schema_test

import (
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/schema"
	"reflect"
	"testing"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/kinds"
)

func TestStringMapScalar_ParseValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected interface{}
	}{
		{
			input:    map[string]interface{}{"key": "val"},
			expected: map[string]interface{}{"key": "val"},
		},
		{
			input:    map[string]string{"a": "b"},
			expected: map[string]string{"a": "b"},
		},
		{
			input:    "key=val",
			expected: nil,
		},
	}

	for _, test := range tests {
		out := schema.StringMapScalar.ParseValue(test.input)
		if !reflect.DeepEqual(out, test.expected) {
			t.Errorf("ParseValue(%v) = %v; want %v", test.input, out, test.expected)
		}
	}
}

func TestStringMapScalar_ParseLiteral(t *testing.T) {
	tests := []struct {
		name     string
		input    ast.Value
		expected interface{}
	}{
		{
			name: "valid_object_value",
			input: &ast.ObjectValue{
				Kind: kinds.ObjectValue,
				Fields: []*ast.ObjectField{
					{
						Name:  &ast.Name{Value: "key"},
						Value: &ast.StringValue{Kind: kinds.StringValue, Value: "val"},
					},
					{
						Name:  &ast.Name{Value: "key2"},
						Value: &ast.StringValue{Kind: kinds.StringValue, Value: "val2"},
					},
				},
			},
			expected: map[string]string{"key": "val", "key2": "val2"},
		},
		{
			name:     "invalid_string_value",
			input:    &ast.StringValue{Kind: kinds.StringValue, Value: "key=val"},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := schema.StringMapScalar.ParseLiteral(tt.input)
			if !reflect.DeepEqual(out, tt.expected) {
				t.Errorf("ParseLiteral() = %v, want %v", out, tt.expected)
			}
		})
	}
}

package schema_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema"

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
		out := schema.StringMapScalarForTest.ParseValue(test.input)
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
			out := schema.StringMapScalarForTest.ParseLiteral(tt.input)
			if !reflect.DeepEqual(out, tt.expected) {
				t.Errorf("ParseLiteral() = %v, want %v", out, tt.expected)
			}
		})
	}
}

func TestSanitizeFieldNameUtil(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid_name",
			input:    "validFieldName",
			expected: "validFieldName",
		},
		{
			name:     "with_dashes",
			input:    "field-name",
			expected: "field_name",
		},
		{
			name:     "starts_with_number",
			input:    "1field",
			expected: "_1field",
		},
		{
			name:     "complex_case",
			input:    "field.name-with$special",
			expected: "field_name_with_special",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := schema.SanitizeFieldNameForTest(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeFieldNameForTest(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGenerateTypeName(t *testing.T) {
	g := schema.GetGatewayForTest(map[string]string{})

	tests := []struct {
		name       string
		typePrefix string
		fieldPath  []string
		expected   string
	}{
		{
			name:       "simple_case",
			typePrefix: "Pod",
			fieldPath:  []string{"spec", "containers"},
			expected:   "Podspeccontainers",
		},
		{
			name:       "empty_field_path",
			typePrefix: "Service",
			fieldPath:  []string{},
			expected:   "Service",
		},
		{
			name:       "single_field",
			typePrefix: "ConfigMap",
			fieldPath:  []string{"data"},
			expected:   "ConfigMapdata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.GenerateTypeNameForTest(tt.typePrefix, tt.fieldPath)
			if got != tt.expected {
				t.Errorf("GenerateTypeNameForTest() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestJSONStringScalar_ProperSerialization(t *testing.T) {
	testObject := map[string]interface{}{
		"name":      "example-config",
		"namespace": "default",
		"labels": map[string]string{
			"hello": "world",
		},
		"annotations": map[string]string{
			"kcp.io/cluster": "root",
		},
	}

	// Test the JSONString scalar serialization
	result := schema.JSONStringScalarForTest.Serialize(testObject)

	if result == nil {
		t.Fatal("JSONStringScalar.Serialize returned nil")
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("JSONStringScalar.Serialize returned %T, expected string", result)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(resultStr), &parsed)
	if err != nil {
		t.Fatalf("Result is not valid JSON: %s\nResult: %s", err, resultStr)
	}

	// Verify the content is preserved
	if parsed["name"] != "example-config" {
		t.Errorf("Name not preserved: got %v, want %v", parsed["name"], "example-config")
	}

	if parsed["namespace"] != "default" {
		t.Errorf("Namespace not preserved: got %v, want %v", parsed["namespace"], "default")
	}

	// Verify it's NOT Go map format
	if len(resultStr) > 10 && resultStr[:4] == "map[" {
		t.Errorf("Result is in Go map format, not JSON: %s", resultStr)
	}

	t.Logf("Proper JSON output: %s", resultStr)
}

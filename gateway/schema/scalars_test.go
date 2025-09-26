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
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "map_string_interface",
			input:    map[string]interface{}{"key": "val"},
			expected: map[string]interface{}{"key": "val"},
		},
		{
			name:     "map_string_string",
			input:    map[string]string{"a": "b"},
			expected: map[string]string{"a": "b"},
		},
		{
			name:     "invalid_string_input",
			input:    "key=val",
			expected: nil,
		},
		{
			name:     "nil_input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "number_input",
			input:    123,
			expected: nil,
		},
		{
			name: "array_of_key_value_objects",
			input: []interface{}{
				map[string]interface{}{"key": "name", "value": "test"},
				map[string]interface{}{"key": "env", "value": "prod"},
			},
			expected: map[string]string{"name": "test", "env": "prod"},
		},
		{
			name: "array_with_invalid_objects",
			input: []interface{}{
				map[string]interface{}{"key": "name", "value": "test"},
				"invalid-item",
				map[string]interface{}{"key": "env", "value": "prod"},
			},
			expected: map[string]string{"name": "test", "env": "prod"},
		},
		{
			name: "array_with_missing_key",
			input: []interface{}{
				map[string]interface{}{"value": "test"},
				map[string]interface{}{"key": "env", "value": "prod"},
			},
			expected: map[string]string{"env": "prod"},
		},
		{
			name: "array_with_missing_value",
			input: []interface{}{
				map[string]interface{}{"key": "name"},
				map[string]interface{}{"key": "env", "value": "prod"},
			},
			expected: map[string]string{"env": "prod"},
		},
		{
			name: "array_with_non_string_key",
			input: []interface{}{
				map[string]interface{}{"key": 123, "value": "test"},
				map[string]interface{}{"key": "env", "value": "prod"},
			},
			expected: map[string]string{"env": "prod"},
		},
		{
			name: "array_with_non_string_value",
			input: []interface{}{
				map[string]interface{}{"key": "name", "value": 123},
				map[string]interface{}{"key": "env", "value": "prod"},
			},
			expected: map[string]string{"env": "prod"},
		},
		{
			name:     "empty_array",
			input:    []interface{}{},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := schema.StringMapScalarForTest.ParseValue(tt.input)
			if !reflect.DeepEqual(out, tt.expected) {
				t.Errorf("ParseValue(%v) = %v; want %v", tt.input, out, tt.expected)
			}
		})
	}
}

func TestStringMapScalar_Serialize(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "map_string_string",
			input:    map[string]string{"key": "value"},
			expected: map[string]string{"key": "value"},
		},
		{
			name:     "map_string_interface",
			input:    map[string]interface{}{"key": "value"},
			expected: map[string]interface{}{"key": "value"},
		},
		{
			name:     "nil_input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "string_input",
			input:    "test",
			expected: "test",
		},
		{
			name:     "number_input",
			input:    123,
			expected: 123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := schema.StringMapScalarForTest.Serialize(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Serialize(%v) = %v; want %v", tt.input, result, tt.expected)
			}
		})
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
			name: "object_value_with_non_string_field",
			input: &ast.ObjectValue{
				Kind: kinds.ObjectValue,
				Fields: []*ast.ObjectField{
					{
						Name:  &ast.Name{Value: "key"},
						Value: &ast.StringValue{Kind: kinds.StringValue, Value: "val"},
					},
					{
						Name:  &ast.Name{Value: "number"},
						Value: &ast.IntValue{Kind: kinds.IntValue, Value: "123"},
					},
				},
			},
			expected: map[string]string{"key": "val", "number": "123"},
		},
		{
			name: "empty_object_value",
			input: &ast.ObjectValue{
				Kind:   kinds.ObjectValue,
				Fields: []*ast.ObjectField{},
			},
			expected: map[string]string{},
		},
		{
			name: "list_value_with_key_value_objects",
			input: &ast.ListValue{
				Kind: kinds.ListValue,
				Values: []ast.Value{
					&ast.ObjectValue{
						Kind: kinds.ObjectValue,
						Fields: []*ast.ObjectField{
							{
								Name:  &ast.Name{Value: "key"},
								Value: &ast.StringValue{Kind: kinds.StringValue, Value: "name"},
							},
							{
								Name:  &ast.Name{Value: "value"},
								Value: &ast.StringValue{Kind: kinds.StringValue, Value: "test"},
							},
						},
					},
					&ast.ObjectValue{
						Kind: kinds.ObjectValue,
						Fields: []*ast.ObjectField{
							{
								Name:  &ast.Name{Value: "key"},
								Value: &ast.StringValue{Kind: kinds.StringValue, Value: "env"},
							},
							{
								Name:  &ast.Name{Value: "value"},
								Value: &ast.StringValue{Kind: kinds.StringValue, Value: "prod"},
							},
						},
					},
				},
			},
			expected: map[string]string{"name": "prod", "env": "prod"},
		},
		{
			name: "list_value_with_invalid_item",
			input: &ast.ListValue{
				Kind: kinds.ListValue,
				Values: []ast.Value{
					&ast.StringValue{Kind: kinds.StringValue, Value: "invalid"},
					&ast.ObjectValue{
						Kind: kinds.ObjectValue,
						Fields: []*ast.ObjectField{
							{
								Name:  &ast.Name{Value: "key"},
								Value: &ast.StringValue{Kind: kinds.StringValue, Value: "name"},
							},
							{
								Name:  &ast.Name{Value: "value"},
								Value: &ast.StringValue{Kind: kinds.StringValue, Value: "test"},
							},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "list_value_with_non_string_key",
			input: &ast.ListValue{
				Kind: kinds.ListValue,
				Values: []ast.Value{
					&ast.ObjectValue{
						Kind: kinds.ObjectValue,
						Fields: []*ast.ObjectField{
							{
								Name:  &ast.Name{Value: "key"},
								Value: &ast.IntValue{Kind: kinds.IntValue, Value: "123"},
							},
							{
								Name:  &ast.Name{Value: "value"},
								Value: &ast.StringValue{Kind: kinds.StringValue, Value: "test"},
							},
						},
					},
				},
			},
			expected: map[string]string{"123": "test"},
		},
		{
			name: "list_value_with_non_string_value",
			input: &ast.ListValue{
				Kind: kinds.ListValue,
				Values: []ast.Value{
					&ast.ObjectValue{
						Kind: kinds.ObjectValue,
						Fields: []*ast.ObjectField{
							{
								Name:  &ast.Name{Value: "key"},
								Value: &ast.StringValue{Kind: kinds.StringValue, Value: "name"},
							},
							{
								Name:  &ast.Name{Value: "value"},
								Value: &ast.IntValue{Kind: kinds.IntValue, Value: "123"},
							},
						},
					},
				},
			},
			expected: map[string]string{"name": "123"},
		},
		{
			name: "empty_list_value",
			input: &ast.ListValue{
				Kind:   kinds.ListValue,
				Values: []ast.Value{},
			},
			expected: map[string]string{},
		},
		{
			name:     "invalid_string_value",
			input:    &ast.StringValue{Kind: kinds.StringValue, Value: "key=val"},
			expected: nil,
		},
		{
			name:     "invalid_int_value",
			input:    &ast.IntValue{Kind: kinds.IntValue, Value: "123"},
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

func TestJSONStringScalar_Serialize(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name: "valid_object",
			input: map[string]interface{}{
				"name":      "example-config",
				"namespace": "default",
				"labels": map[string]string{
					"hello": "world",
				},
			},
			expected: `{"labels":{"hello":"world"},"name":"example-config","namespace":"default"}`,
		},
		{
			name:     "string_input",
			input:    "test-string",
			expected: `"test-string"`,
		},
		{
			name:     "number_input",
			input:    42,
			expected: "42",
		},
		{
			name:     "boolean_input",
			input:    true,
			expected: "true",
		},
		{
			name:     "nil_input",
			input:    nil,
			expected: "null",
		},
		{
			name:     "empty_map",
			input:    map[string]interface{}{},
			expected: "{}",
		},
		{
			name:     "array_input",
			input:    []string{"a", "b", "c"},
			expected: `["a","b","c"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := schema.JSONStringScalarForTest.Serialize(tt.input)

			if result == nil {
				t.Fatal("JSONStringScalar.Serialize returned nil")
			}

			resultStr, ok := result.(string)
			if !ok {
				t.Fatalf("JSONStringScalar.Serialize returned %T, expected string", result)
			}

			// For complex objects, we can't guarantee exact string match due to map ordering
			// So we parse both and compare
			if tt.name == "valid_object" {
				var expectedParsed, resultParsed map[string]interface{}
				err1 := json.Unmarshal([]byte(tt.expected), &expectedParsed)
				err2 := json.Unmarshal([]byte(resultStr), &resultParsed)

				if err1 != nil || err2 != nil {
					t.Fatalf("Failed to parse JSON: expected err=%v, result err=%v", err1, err2)
				}

				if !reflect.DeepEqual(expectedParsed, resultParsed) {
					t.Errorf("JSONStringScalar.Serialize() = %v, want %v", resultParsed, expectedParsed)
				}
			} else {
				if resultStr != tt.expected {
					t.Errorf("JSONStringScalar.Serialize() = %q, want %q", resultStr, tt.expected)
				}
			}
		})
	}
}

func TestJSONStringScalar_Serialize_MarshalError(t *testing.T) {
	// Test with a value that cannot be marshaled to JSON (e.g., function, channel)
	invalidInput := make(chan int)

	result := schema.JSONStringScalarForTest.Serialize(invalidInput)

	if result != "{}" {
		t.Errorf("JSONStringScalar.Serialize() with invalid input = %q, want %q", result, "{}")
	}
}

func TestJSONStringScalar_ParseValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "valid_json_string",
			input:    `{"name":"test","value":42}`,
			expected: map[string]interface{}{"name": "test", "value": float64(42)},
		},
		{
			name:     "valid_json_array",
			input:    `["a","b","c"]`,
			expected: []interface{}{"a", "b", "c"},
		},
		{
			name:     "valid_json_number",
			input:    `123`,
			expected: float64(123),
		},
		{
			name:     "valid_json_boolean",
			input:    `true`,
			expected: true,
		},
		{
			name:     "valid_json_null",
			input:    `null`,
			expected: nil,
		},
		{
			name:     "invalid_json_string",
			input:    `{"invalid": json}`,
			expected: nil,
		},
		{
			name:     "non_string_input",
			input:    123,
			expected: nil,
		},
		{
			name:     "empty_string",
			input:    "",
			expected: nil,
		},
		{
			name:     "nil_input",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := schema.JSONStringScalarForTest.ParseValue(tt.input)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("JSONStringScalar.ParseValue() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestJSONStringScalar_ParseLiteral(t *testing.T) {
	tests := []struct {
		name     string
		input    ast.Value
		expected interface{}
	}{
		{
			name: "valid_json_string",
			input: &ast.StringValue{
				Kind:  kinds.StringValue,
				Value: `{"name":"test","value":42}`,
			},
			expected: map[string]interface{}{"name": "test", "value": float64(42)},
		},
		{
			name: "valid_json_array",
			input: &ast.StringValue{
				Kind:  kinds.StringValue,
				Value: `["a","b","c"]`,
			},
			expected: []interface{}{"a", "b", "c"},
		},
		{
			name: "invalid_json_string",
			input: &ast.StringValue{
				Kind:  kinds.StringValue,
				Value: `{"invalid": json}`,
			},
			expected: nil,
		},
		{
			name: "non_string_ast_value",
			input: &ast.IntValue{
				Kind:  kinds.IntValue,
				Value: "123",
			},
			expected: nil,
		},
		{
			name: "empty_json_string",
			input: &ast.StringValue{
				Kind:  kinds.StringValue,
				Value: "",
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := schema.JSONStringScalarForTest.ParseLiteral(tt.input)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("JSONStringScalar.ParseLiteral() = %v, want %v", result, tt.expected)
			}
		})
	}
}

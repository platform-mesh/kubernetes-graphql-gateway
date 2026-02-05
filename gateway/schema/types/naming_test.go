package types_test

import (
	"testing"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema/types"
)

func TestSanitizeFieldName(t *testing.T) {
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
		{
			name:     "with_underscore",
			input:    "_privateField",
			expected: "_privateField",
		},
		{
			name:     "all_invalid_chars",
			input:    "!@#$%",
			expected: "_____", // 5 special chars become 5 underscores
		},
		{
			name:     "empty_string",
			input:    "",
			expected: "_", // Empty string gets underscore prepended
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := types.SanitizeFieldName(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeFieldName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGenerateTypeName(t *testing.T) {
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
		{
			name:       "nested_path",
			typePrefix: "Deployment",
			fieldPath:  []string{"spec", "template", "spec", "containers"},
			expected:   "Deploymentspectemplatespeccontainers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := types.GenerateTypeName(tt.typePrefix, tt.fieldPath)
			if got != tt.expected {
				t.Errorf("GenerateTypeName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

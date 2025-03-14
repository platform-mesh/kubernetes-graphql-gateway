package resolver

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSanitizeGroupName(t *testing.T) {
	r := &Service{
		groupNames: make(map[string]string),
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty_string", "", "core"},
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
			assert.Equal(t, tt.input, r.groupNames[result], "The original group name should be stored correctly")
		})
	}
}

func TestGetOriginalGroupName(t *testing.T) {
	r := &Service{
		groupNames: map[string]string{
			"group1": "originalGroup1",
			"group2": "originalGroup2",
		},
	}

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
			result := r.getOriginalGroupName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

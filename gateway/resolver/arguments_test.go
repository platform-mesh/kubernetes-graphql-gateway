package resolver_test

import (
	"errors"
	"testing"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/stretchr/testify/assert"
)

func TestGetStrArg(t *testing.T) {
	tests := []struct {
		name  string
		args  map[string]any
		error error
	}{
		{
			name: "invalid_type_ERROR",
			args: map[string]any{
				"arg1": false,
			},
			error: errors.New("invalid type for argument: arg1"),
		},
		{
			name: "empty_value_ERROR",
			args: map[string]any{
				"arg1": "",
			},
			error: errors.New("empty value for argument: arg1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolver.GetStringArg(tt.args, "arg1", true)
			if tt.error != nil {
				assert.EqualError(t, err, tt.error.Error())
			}
		})
	}
}

func TestGetBoolArg(t *testing.T) {
	tests := []struct {
		name  string
		args  map[string]any
		error error
	}{
		{
			name:  "missing_required_argument_ERROR",
			args:  map[string]any{},
			error: errors.New("missing required argument: arg1"),
		},
		{
			name: "invalid_type_ERROR",
			args: map[string]any{
				"arg1": "MUST_BE_BOOL",
			},
			error: errors.New("invalid type for argument: arg1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolver.GetBoolArg(tt.args, "arg1", true)
			if tt.error != nil {
				assert.EqualError(t, err, tt.error.Error())
			}
		})
	}
}

package apischema

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertJSON_InvalidInput tests the ConvertJSON function with invalid input.
// It checks if the function returns the expected error when given invalid JSON data.
func TestConvertJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantErr error
		check   func(defs map[string]map[string]any) error
	}{
		{
			name:    "invalid_JSON",
			input:   "not a json",
			wantErr: ErrUnmarshalJSON,
		},
		{
			name: "remove_defaults_and_rewrite_refs",
			input: map[string]any{
				"components": map[string]any{
					"schemas": map[string]any{
						"Foo": map[string]any{
							"default": map[string]any{},
							"allOf":   []any{map[string]any{"$ref": "#/components/schemas/Bar"}},
							"properties": map[string]any{
								"nested": map[string]any{
									"allOf": []any{map[string]any{"$ref": "#/components/schemas/Baz"}},
								},
							},
						},
					},
				},
			},
			wantErr: nil,
			check: func(defs map[string]map[string]any) error {
				foo, ok := defs["Foo"]
				if !ok {
					return errors.New("missing Foo definition")
				}
				if _, exists := foo["default"]; exists {
					return errors.New("default key should be removed")
				}
				ref, ok := foo["$ref"].(string)
				if !ok || !strings.Contains(ref, "definitions/Bar") {
					return errors.New("invalid $ref for Foo: " + ref)
				}
				props, ok := foo["properties"].(map[string]any)
				if !ok {
					return errors.New("properties not a map")
				}
				nested, ok := props["nested"].(map[string]any)
				if !ok {
					return errors.New("nested not a map")
				}
				nref, ok := nested["$ref"].(string)
				if !ok || !strings.Contains(nref, "definitions/Baz") {
					return errors.New("invalid nested $ref: " + nref)
				}
				return nil
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var raw []byte
			var err error
			switch in := tc.input.(type) {
			case string:
				raw = []byte(in)
			default:
				raw, err = json.Marshal(in)
				require.NoError(t, err, "failed to marshal input %s", tc.name)
			}
			out, err := ConvertJSON(raw)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr, "error mismatch")
				return
			}
			assert.NoError(t, err, "unexpected error")
			var w v2RootWrapper
			err = json.Unmarshal(out, &w)
			require.NoErrorf(t, err, "%s: failed to unmarshal output", tc.name)

			if tc.check != nil {
				defs := map[string]map[string]any{}
				for k, v := range w.Definitions {
					if m, ok := v.(map[string]any); ok {
						defs[k] = m
					}
				}
				err := tc.check(defs)
				assert.NoErrorf(t, err, "%s: check failed", tc.name)
			}
		})
	}
}

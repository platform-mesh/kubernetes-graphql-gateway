package apischema

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
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
			name:    "invalid JSON",
			input:   "not a json",
			wantErr: ErrUnmarshalJSON,
		},
		{
			name: "remove defaults and rewrite refs",
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
				if err != nil {
					t.Fatalf("failed to marshal input %s: %v", tc.name, err)
				}
			}
			out, err := ConvertJSON(raw)
			if tc.wantErr != nil {
				if err == nil || !errors.Is(err, tc.wantErr) {
					t.Fatalf("%s: expected error %v, got %v", tc.name, tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", tc.name, err)
			}
			var w v2RootWrapper
			if err := json.Unmarshal(out, &w); err != nil {
				t.Fatalf("%s: failed to unmarshal output: %v", tc.name, err)
			}
			if tc.check != nil {
				defs := map[string]map[string]any{}
				for k, v := range w.Definitions {
					if m, ok := v.(map[string]any); ok {
						defs[k] = m
					}
				}
				if err := tc.check(defs); err != nil {
					t.Errorf("%s: check failed: %v", tc.name, err)
				}
			}
		})
	}
}

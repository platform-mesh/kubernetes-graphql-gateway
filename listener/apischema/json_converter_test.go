package apischema

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestConvertJSON_InvalidInput(t *testing.T) {
	_, err := ConvertJSON([]byte("not a json"))
	if !errors.Is(err, ErrUnmarshalJSON) {
		t.Errorf("expected ErrUnmarshalJSON, got %v", err)
	}
}

func TestConvertJSON_Transforms(t *testing.T) {
	input := []byte(`{
		"components": {
			"schemas": {
				"Foo": {
					"default": {},
					"allOf": [{ "$ref": "#/components/schemas/Bar" }],
					"properties": {
						"nested": {
							"allOf": [{ "$ref": "#/components/schemas/Baz" }]
						}
					}
				}
			}
		}
	}`)

	expected := `{
		"definitions": {
			"Foo": {
				"$ref": "#/definitions/Bar",
				"properties": {
					"nested": {
						"$ref": "#/definitions/Baz"
					}
				}
			}
		}
	}`

	out, err := ConvertJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got, want map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatalf("unmarshal expected: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("output mismatch:\n got: %v\nwant: %v", got, want)
	}
}

package apischema_test

import (
	"encoding/json"
	"testing"

	"github.com/openmfp/kubernetes-graphql-gateway/listener/apischema"
	"github.com/stretchr/testify/assert"
)

func TestConvertJSON_InvalidInput(t *testing.T) {
	_, err := apischema.ConvertJSON([]byte("not a json"))
	assert.ErrorIs(t, err, apischema.ErrUnmarshalJSON)
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

	out, err := apischema.ConvertJSON(input)
	assert.NoError(t, err)

	var got, want map[string]any
	assert.NoError(t, json.Unmarshal(out, &got), "unmarshal output")
	assert.NoError(t, json.Unmarshal([]byte(expected), &want), "unmarshal expected")
	assert.Equal(t, len(want), len(got), "output length mismatch")
	assert.Equal(t, want, got, "output mismatch")
}

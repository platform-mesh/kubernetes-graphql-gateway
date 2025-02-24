package apischema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type v3Wrapper struct {
	Schemas map[string]any `json:"schemas"`
}

type v3RootWrapper struct {
	Components v3Wrapper `json:"components"`
}

type v2RootWrapper struct {
	Definitions map[string]any `json:"definitions"`
}

func ConvertJSON(v3JSON []byte) ([]byte, error) {
	data := &v3RootWrapper{}
	if err := json.Unmarshal(v3JSON, data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	v2JSON := parseJSON(data.Components.Schemas)
	v2, ok := v2JSON.(map[string]any)
	if !ok {
		return nil, errors.New("failed to validate converted JSON")
	}
	buf := &bytes.Buffer{}
	e := json.NewEncoder(buf)
	e.SetEscapeHTML(false)
	encErr := e.Encode(&v2RootWrapper{
		Definitions: v2,
	})
	return buf.Bytes(), encErr
}

func parseJSON(data any) any {

	v, ok := data.(map[string]any)
	if !ok {
		return data
	}
	if defaultVal, exists := v["default"]; exists {
		if defaultMap, ok := defaultVal.(map[string]interface{}); ok && len(defaultMap) == 0 {
			delete(v, "default")
		}
	}
	if allOf, exists := v["allOf"]; exists {
		if refs, ok := allOf.([]any); ok && len(refs) == 1 {
			if refObj, ok := refs[0].(map[string]any); ok {
				if ref, ok := refObj["$ref"].(string); ok {
					// Replace "allOf" with "$ref"
					if strings.Contains(ref, "components/schemas") {
						r := strings.NewReplacer("components/schemas", "definitions")
						ref = r.Replace(ref)
					}
					v["$ref"] = ref
					delete(v, "allOf")
				}
			}
		}
	}
	for key, val := range v {
		v[key] = parseJSON(val)
	}
	return v
}

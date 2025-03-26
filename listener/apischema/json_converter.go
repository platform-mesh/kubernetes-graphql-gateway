package apischema

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

var (
	ErrFailedToValidateConvertedJson = errors.New("failed to validate converted JSON")
	ErrUnmarshalJSON                 = errors.New("failed to unmarshal JSON")
	ErrEncodeJSON                    = errors.New("failed to encode JSON")
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
		return nil, errors.Join(ErrUnmarshalJSON, err)
	}

	v2JSON := parseJSON(data.Components.Schemas)
	v2, ok := v2JSON.(map[string]any)
	if !ok {
		return nil, ErrFailedToValidateConvertedJson
	}
	buf := &bytes.Buffer{}
	e := json.NewEncoder(buf)
	e.SetEscapeHTML(false)
	encErr := e.Encode(&v2RootWrapper{
		Definitions: v2,
	})
	if encErr != nil {
		return nil, errors.Join(ErrEncodeJSON, encErr)
	}
	return buf.Bytes(), nil
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

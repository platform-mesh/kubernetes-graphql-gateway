package schema

import (
	"encoding/json"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

var jsonStringScalar = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "JSONString",
	Description: "A JSON-serialized string representation of any object.",
	Serialize: func(value interface{}) interface{} {
		// Convert the value to JSON string
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			// Fallback to empty JSON object if marshaling fails
			return "{}"
		}
		return string(jsonBytes)
	},
	ParseValue: func(value interface{}) interface{} {
		if str, ok := value.(string); ok {
			var result interface{}
			err := json.Unmarshal([]byte(str), &result)
			if err != nil {
				return nil // Invalid JSON
			}
			return result
		}
		return nil
	},
	ParseLiteral: func(valueAST ast.Value) interface{} {
		if value, ok := valueAST.(*ast.StringValue); ok {
			var result interface{}
			err := json.Unmarshal([]byte(value.Value), &result)
			if err != nil {
				return nil // Invalid JSON
			}
			return result
		}
		return nil
	},
})

var stringMapScalar = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "StringMapInput",
	Description: "Input type for a map from strings to strings.",
	Serialize: func(value interface{}) interface{} {
		return value
	},
	ParseValue: func(value interface{}) interface{} {
		switch val := value.(type) {
		case map[string]interface{}, map[string]string:
			return val
		default:
			// Added this to handle GraphQL variables
			if arr, ok := value.([]interface{}); ok {
				result := make(map[string]string)
				for _, item := range arr {
					if obj, ok := item.(map[string]interface{}); ok {
						if key, keyOk := obj["key"].(string); keyOk {
							if val, valOk := obj["value"].(string); valOk {
								result[key] = val
							}
						}
					}
				}
				return result
			}
			return nil // to tell GraphQL that the value is invalid
		}
	},
	ParseLiteral: func(valueAST ast.Value) any {
		switch value := valueAST.(type) {
		case *ast.ListValue:
			result := make(map[string]string)
			for _, item := range value.Values {
				obj, ok := item.(*ast.ObjectValue)
				if !ok {
					return nil
				}

				for _, field := range obj.Fields {
					switch field.Name.Value {
					case "key":
						if key, ok := field.Value.GetValue().(string); ok {
							result[key] = ""
						}
					case "value":
						if val, ok := field.Value.GetValue().(string); ok {
							for key := range result {
								result[key] = val
							}
						}
					}
				}
			}

			return result
		case *ast.ObjectValue:
			result := map[string]string{}
			for _, field := range value.Fields {
				if strValue, ok := field.Value.GetValue().(string); ok {
					result[field.Name.Value] = strValue
				}
			}
			return result
		default:
			return nil // to tell GraphQL that the value is invalid
		}
	},
})

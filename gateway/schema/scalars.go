package schema

import (
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

var stringMapScalar = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "StringMap",
	Description: "A map from strings to strings.",
	Serialize: func(value interface{}) interface{} {
		return value
	},
	ParseValue: func(value interface{}) interface{} {
		switch val := value.(type) {
		case map[string]interface{}, map[string]string:
			return val
		default:
			return nil // to tell GraphQL that the value is invalid
		}
	},
	ParseLiteral: func(valueAST ast.Value) interface{} {
		switch value := valueAST.(type) {
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

package gateway

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
		return value
	},
	ParseLiteral: func(valueAST ast.Value) interface{} {
		result := map[string]string{}
		switch value := valueAST.(type) {
		case *ast.ObjectValue:
			for _, field := range value.Fields {
				if strValue, ok := field.Value.GetValue().(string); ok {
					result[field.Name.Value] = strValue
				}
			}
		}
		return result
	},
})

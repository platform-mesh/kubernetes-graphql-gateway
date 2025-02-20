package resolver

import "github.com/graphql-go/graphql"

const (
	LabelSelectorArg  = "labelselector"
	NameArg           = "name"
	NamespaceArg      = "namespace"
	ObjectArg         = "object"
	SubscribeToAllArg = "subscribeToAll"
)

// FieldConfigArgumentsBuilder helps construct GraphQL field config arguments
type FieldConfigArgumentsBuilder struct {
	arguments graphql.FieldConfigArgument
}

// NewFieldConfigArguments initializes a new builder
func NewFieldConfigArguments() *FieldConfigArgumentsBuilder {
	return &FieldConfigArgumentsBuilder{
		arguments: graphql.FieldConfigArgument{},
	}
}

func (b *FieldConfigArgumentsBuilder) WithNameArg() *FieldConfigArgumentsBuilder {
	b.arguments[NameArg] = &graphql.ArgumentConfig{
		Type:        graphql.NewNonNull(graphql.String),
		Description: "The name of the object",
	}
	return b
}

func (b *FieldConfigArgumentsBuilder) WithNamespaceArg() *FieldConfigArgumentsBuilder {
	b.arguments[NamespaceArg] = &graphql.ArgumentConfig{
		Type:        graphql.String,
		Description: "The namespace in which to search for the objects",
	}
	return b
}

func (b *FieldConfigArgumentsBuilder) WithLabelSelectorArg() *FieldConfigArgumentsBuilder {
	b.arguments[LabelSelectorArg] = &graphql.ArgumentConfig{
		Type:        graphql.String,
		Description: "A label selector to filter the objects by",
	}
	return b
}

func (b *FieldConfigArgumentsBuilder) WithObjectArg(resourceInputType *graphql.InputObject) *FieldConfigArgumentsBuilder {
	b.arguments[ObjectArg] = &graphql.ArgumentConfig{
		Type:        graphql.NewNonNull(resourceInputType),
		Description: "The object to create or update",
	}
	return b
}

func (b *FieldConfigArgumentsBuilder) WithSubscribeToAllArg() *FieldConfigArgumentsBuilder {
	b.arguments[SubscribeToAllArg] = &graphql.ArgumentConfig{
		Type:         graphql.Boolean,
		DefaultValue: false,
		Description:  "If true, events will be emitted on every field change",
	}
	return b
}

// Complete returns the constructed arguments
func (b *FieldConfigArgumentsBuilder) Complete() graphql.FieldConfigArgument {
	return b.arguments
}

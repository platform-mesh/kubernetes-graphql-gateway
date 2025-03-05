package resolver

import (
	"errors"
	"maps"

	"github.com/graphql-go/graphql"
	"github.com/rs/zerolog/log"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

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

// Complete returns the constructed arguments and dereferences the builder
func (b *FieldConfigArgumentsBuilder) Complete() graphql.FieldConfigArgument {
	return maps.Clone(b.arguments)
}

func getStringArg(args map[string]interface{}, key string, required bool) (string, error) {
	val, exists := args[key]
	if !exists {
		if required {
			err := errors.New("missing required argument: " + key)
			log.Error().Err(err).Msg(key + " argument is required")
			return "", err
		}

		return "", nil
	}

	str, ok := val.(string)
	if !ok {
		err := errors.New("invalid type for argument: " + key)
		log.Error().Err(err).Msg(key + " argument must be a string")
		return "", err
	}

	if str == "" {
		err := errors.New("empty value for argument: " + key)
		log.Error().Err(err).Msg(key + " argument cannot be empty")
		return "", err
	}

	return str, nil
}

func getBoolArg(args map[string]interface{}, key string, required bool) (bool, error) {
	val, exists := args[key]
	if !exists {
		if required {
			err := errors.New("missing required argument: " + key)
			log.Error().Err(err).Msg(key + " argument is required")
			return false, err
		}

		return false, nil
	}

	res, ok := val.(bool)
	if !ok {
		err := errors.New("invalid type for argument: " + key)
		log.Error().Err(err).Msg(key + " argument must be a bool")
		return false, err
	}

	return res, nil
}

func isResourceNamespaceScoped(resourceScope apiextensionsv1.ResourceScope) bool {
	return resourceScope == apiextensionsv1.NamespaceScoped
}

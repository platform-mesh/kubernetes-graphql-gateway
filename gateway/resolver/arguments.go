package resolver

import (
	"errors"
	"maps"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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
	SortByArg         = "sortBy"
	DryRunArg         = "dryRun"
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

func (b *FieldConfigArgumentsBuilder) WithName() *FieldConfigArgumentsBuilder {
	b.arguments[NameArg] = &graphql.ArgumentConfig{
		Type:        graphql.NewNonNull(graphql.String),
		Description: "The name of the object",
	}
	return b
}

func (b *FieldConfigArgumentsBuilder) WithNamespace() *FieldConfigArgumentsBuilder {
	b.arguments[NamespaceArg] = &graphql.ArgumentConfig{
		Type:        graphql.String,
		Description: "The namespace in which to search for the objects",
	}

	return b
}

func (b *FieldConfigArgumentsBuilder) WithLabelSelector() *FieldConfigArgumentsBuilder {
	b.arguments[LabelSelectorArg] = &graphql.ArgumentConfig{
		Type:        graphql.String,
		Description: "A label selector to filter the objects by",
	}
	return b
}

func (b *FieldConfigArgumentsBuilder) WithObject(resourceInputType *graphql.InputObject) *FieldConfigArgumentsBuilder {
	b.arguments[ObjectArg] = &graphql.ArgumentConfig{
		Type:        graphql.NewNonNull(resourceInputType),
		Description: "The object to create or update",
	}
	return b
}

func (b *FieldConfigArgumentsBuilder) WithDryRun() *FieldConfigArgumentsBuilder {
	b.arguments[DryRunArg] = &graphql.ArgumentConfig{
		Type:        graphql.NewList(graphql.String),
		Description: "If true, the object will not be persisted",
	}
	return b
}

func (b *FieldConfigArgumentsBuilder) WithSubscribeToAll() *FieldConfigArgumentsBuilder {
	b.arguments[SubscribeToAllArg] = &graphql.ArgumentConfig{
		Type:         graphql.Boolean,
		DefaultValue: false,
		Description:  "If true, events will be emitted on every field change",
	}
	return b
}

func (b *FieldConfigArgumentsBuilder) WithSortBy() *FieldConfigArgumentsBuilder {
	b.arguments[SortByArg] = &graphql.ArgumentConfig{
		Type:         graphql.String,
		Description:  "The field to sort the results by",
		DefaultValue: "metadata.name",
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

func validateSortBy(items []unstructured.Unstructured, fieldPath string) error {
	if len(items) == 0 {
		return nil // No items to validate against, assume valid
	}

	sample := items[0]
	segments := strings.Split(fieldPath, ".")

	_, found, err := unstructured.NestedFieldNoCopy(sample.Object, segments...)
	if !found {
		return errors.New("specified sortBy field does not exist")
	}
	if err != nil {
		return errors.Join(errors.New("error accessing specified sortBy field"), err)
	}

	return nil
}

func getDryRunArg(args map[string]interface{}, key string, required bool) ([]string, error) {
	val, exists := args[key]
	if !exists {
		if required {
			err := errors.New("missing required argument: " + key)
			log.Error().Err(err).Msg(key + " argument is required")
			return nil, err
		}
		return nil, nil
	}

	switch v := val.(type) {
	case []interface{}:
		result := make([]string, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				err := errors.New("invalid type in dryRun list: expected string")
				log.Error().Err(err).Msg("dryRun argument must be a list of strings")
				return nil, err
			}
			result[i] = str
		}
		return result, nil
	case nil:
		return nil, nil
	default:
		err := errors.New("invalid type for dryRun argument: expected list of strings")
		log.Error().Err(err).Msg("dryRun argument must be a list of strings")
		return nil, err
	}
}

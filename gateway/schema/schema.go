package schema

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-openapi/spec"
	"github.com/gobuffalo/flect"
	"github.com/graphql-go/graphql"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/kubernetes-graphql-gateway/common"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/resolver"
)

type Provider interface {
	GetSchema() *graphql.Schema
}

type Gateway struct {
	log           *logger.Logger
	resolver      resolver.Provider
	graphqlSchema graphql.Schema

	definitions spec.Definitions

	// typesCache stores generated GraphQL object types(fields) to prevent redundant repeated generation.
	typesCache map[string]*graphql.Object
	// inputTypesCache stores generated GraphQL input object types(input fields) to prevent redundant repeated generation.
	inputTypesCache map[string]*graphql.InputObject
	// Prevents naming conflict in case of the same Kind name in different groups/versions
	typeNameRegistry map[string]string // map[Kind]GroupVersion

	// categoryRegistry stores resources by category for typeByCategory query
	typeByCategory map[string][]resolver.TypeByCategory
}

func New(log *logger.Logger, definitions spec.Definitions, resolverProvider resolver.Provider) (*Gateway, error) {
	g := &Gateway{
		log:              log,
		resolver:         resolverProvider,
		definitions:      definitions,
		typesCache:       make(map[string]*graphql.Object),
		inputTypesCache:  make(map[string]*graphql.InputObject),
		typeNameRegistry: make(map[string]string),
		typeByCategory:   make(map[string][]resolver.TypeByCategory),
	}

	err := g.generateGraphqlSchema()

	return g, err
}

func (g *Gateway) GetSchema() *graphql.Schema {
	return &g.graphqlSchema
}

func (g *Gateway) generateGraphqlSchema() error {
	rootQueryFields := graphql.Fields{}
	rootMutationFields := graphql.Fields{}
	rootSubscriptionFields := graphql.Fields{}

	for group, groupedResources := range g.getDefinitionsByGroup(g.definitions) {
		g.processGroupedResources(
			group,
			groupedResources,
			rootQueryFields,
			rootMutationFields,
			rootSubscriptionFields,
		)
	}

	g.AddTypeByCategoryQuery(rootQueryFields)

	newSchema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name:   "PrivateNameForQuery", // we must keep those name unique to avoid collision with objects having the same names
			Fields: rootQueryFields,
		}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{
			Name:   "PrivateNameForMutation",
			Fields: rootMutationFields,
		}),
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name:   "PrivateNameForSubscription",
			Fields: rootSubscriptionFields,
		}),
	})

	if err != nil {
		g.log.Error().Err(err).Msg("Error creating GraphQL schema")
		return err
	}

	g.graphqlSchema = newSchema

	return nil
}

func (g *Gateway) processGroupedResources(
	group string,
	groupedResources spec.Definitions,
	rootQueryFields,
	rootMutationFields,
	rootSubscriptionFields graphql.Fields,
) {
	queryGroupType := graphql.NewObject(graphql.ObjectConfig{
		Name:   group + "Query",
		Fields: graphql.Fields{},
	})

	mutationGroupType := graphql.NewObject(graphql.ObjectConfig{
		Name:   group + "Mutation",
		Fields: graphql.Fields{},
	})

	for resourceKey, resourceScheme := range groupedResources {
		g.processSingleResource(
			resourceKey,
			resourceScheme,
			queryGroupType,
			mutationGroupType,
			rootSubscriptionFields,
		)
	}

	if len(queryGroupType.Fields()) > 0 {
		rootQueryFields[group] = &graphql.Field{
			Type:    queryGroupType,
			Resolve: g.resolver.CommonResolver(),
		}
	}

	if len(mutationGroupType.Fields()) > 0 {
		rootMutationFields[group] = &graphql.Field{
			Type:    mutationGroupType,
			Resolve: g.resolver.CommonResolver(),
		}
	}
}

func (g *Gateway) processSingleResource(
	resourceKey string,
	resourceScheme spec.Schema,
	queryGroupType, mutationGroupType *graphql.Object,
	rootSubscriptionFields graphql.Fields,
) {
	gvk, err := g.getGroupVersionKind(resourceKey)
	if err != nil {
		g.log.Debug().Err(err).Msg("Failed to get group version kind")
		return
	}

	if strings.HasSuffix(gvk.Kind, "List") {
		// Skip List resources
		return
	}

	resourceScope, err := g.getScope(resourceKey)
	if err != nil {
		g.log.Error().Err(err).Str("resource", resourceKey).Msg("Error getting resourceScope")
		return
	}

	err = g.storeCategory(resourceKey, gvk, resourceScope)
	if err != nil {
		g.log.Debug().Err(err).Str("resource", resourceKey).Msg("Error storing category")
	}

	singular, plural := g.getNames(gvk)

	// Generate both fields and inputFields
	fields, inputFields, err := g.generateGraphQLFields(&resourceScheme, singular, []string{}, make(map[string]bool))
	if err != nil {
		g.log.Error().Err(err).Str("resource", singular).Msg("Error generating fields")
		return
	}

	if len(fields) == 0 {
		g.log.Debug().Str("resource", singular).Msg("No fields found")
		return
	}

	resourceType := graphql.NewObject(graphql.ObjectConfig{
		Name:   singular,
		Fields: fields,
	})

	resourceInputType := graphql.NewInputObject(graphql.InputObjectConfig{
		Name:   singular + "Input",
		Fields: inputFields,
	})

	listArgsBuilder := resolver.NewFieldConfigArguments().
		WithLabelSelector().
		WithSortBy()

	itemArgsBuilder := resolver.NewFieldConfigArguments().WithName()

	creationMutationArgsBuilder := resolver.NewFieldConfigArguments().WithObject(resourceInputType).WithDryRun()

	if resourceScope == apiextensionsv1.NamespaceScoped {
		listArgsBuilder.WithNamespace()
		itemArgsBuilder.WithNamespace()
		creationMutationArgsBuilder.WithNamespace()
	}

	listArgs := listArgsBuilder.Complete()
	itemArgs := itemArgsBuilder.Complete()
	creationMutationArgs := creationMutationArgsBuilder.Complete()

	queryGroupType.AddFieldConfig(plural, &graphql.Field{
		Type:    graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(resourceType))),
		Args:    listArgs,
		Resolve: g.resolver.ListItems(*gvk, resourceScope),
	})

	queryGroupType.AddFieldConfig(singular, &graphql.Field{
		Type:    graphql.NewNonNull(resourceType),
		Args:    itemArgs,
		Resolve: g.resolver.GetItem(*gvk, resourceScope),
	})

	queryGroupType.AddFieldConfig(singular+"Yaml", &graphql.Field{
		Type:    graphql.NewNonNull(graphql.String),
		Args:    itemArgs,
		Resolve: g.resolver.GetItemAsYAML(*gvk, resourceScope),
	})

	// Mutation definitions
	mutationGroupType.AddFieldConfig("create"+singular, &graphql.Field{
		Type:    resourceType,
		Args:    creationMutationArgs,
		Resolve: g.resolver.CreateItem(*gvk, resourceScope),
	})

	mutationGroupType.AddFieldConfig("update"+singular, &graphql.Field{
		Type:    resourceType,
		Args:    creationMutationArgsBuilder.WithName().Complete(),
		Resolve: g.resolver.UpdateItem(*gvk, resourceScope),
	})

	mutationGroupType.AddFieldConfig("delete"+singular, &graphql.Field{
		Type:    graphql.Boolean,
		Args:    itemArgsBuilder.WithDryRun().Complete(),
		Resolve: g.resolver.DeleteItem(*gvk, resourceScope),
	})

	subscriptionSingular := strings.ToLower(fmt.Sprintf("%s_%s", gvk.Group, singular))
	rootSubscriptionFields[subscriptionSingular] = &graphql.Field{
		Type: addErrorFieldToGraphqlObject(resourceType),
		Args: itemArgsBuilder.
			WithSubscribeToAll().
			Complete(),
		Resolve:     g.resolver.CommonResolver(),
		Subscribe:   g.resolver.SubscribeItem(*gvk, resourceScope),
		Description: fmt.Sprintf("Subscribe to changes of %s", singular),
	}

	subscriptionPlural := strings.ToLower(fmt.Sprintf("%s_%s", gvk.Group, plural))
	rootSubscriptionFields[subscriptionPlural] = &graphql.Field{
		Type: graphql.NewList(addErrorFieldToGraphqlObject(resourceType)),
		Args: listArgsBuilder.
			WithSubscribeToAll().
			Complete(),
		Resolve:     g.resolver.CommonResolver(),
		Subscribe:   g.resolver.SubscribeItems(*gvk, resourceScope),
		Description: fmt.Sprintf("Subscribe to changes of %s", plural),
	}
}

func (g *Gateway) getNames(gvk *schema.GroupVersionKind) (singular string, plural string) {
	kind := gvk.Kind
	singular = kind
	plural = flect.Pluralize(singular)

	// Check if the kind name has already been used for a different group/version
	if existingGroupVersion, exists := g.typeNameRegistry[kind]; exists {
		if existingGroupVersion != gvk.GroupVersion().String() {
			// Conflict detected, append group and version to the kind for uniqueness
			// we don't add new entry to the registry, because we already have one with the same kind
			group := strings.ReplaceAll(gvk.Group, ".", "") // dots are allowed in k8s group, but not in graphql
			singular = strings.Join([]string{kind, group, gvk.Version}, "_")
			plural = strings.Join([]string{plural, group, gvk.Version}, "_")
		}
	} else {
		// No conflict, register the kind with its group and version
		g.typeNameRegistry[kind] = gvk.GroupVersion().String()
	}

	return singular, plural
}

func (g *Gateway) getDefinitionsByGroup(filteredDefinitions spec.Definitions) map[string]spec.Definitions {
	groups := map[string]spec.Definitions{}
	for key, definition := range filteredDefinitions {
		gvk, err := g.getGroupVersionKind(key)
		if err != nil {
			g.log.Debug().Err(err).Str("resourceKey", key).Msg("Failed to get group version kind")
			continue
		}

		if _, ok := groups[gvk.Group]; !ok {
			groups[gvk.Group] = spec.Definitions{}
		}

		groups[gvk.Group][key] = definition
	}

	return groups
}

func (g *Gateway) generateGraphQLFields(resourceScheme *spec.Schema, typePrefix string, fieldPath []string, processingTypes map[string]bool) (graphql.Fields, graphql.InputObjectConfigFieldMap, error) {
	fields := graphql.Fields{}
	inputFields := graphql.InputObjectConfigFieldMap{}

	for fieldName, fieldSpec := range resourceScheme.Properties {
		sanitizedFieldName := sanitizeFieldName(fieldName)
		currentFieldPath := append(fieldPath, fieldName)

		fieldType, inputFieldType, err := g.convertSwaggerTypeToGraphQL(fieldSpec, typePrefix, currentFieldPath, processingTypes)
		if err != nil {
			return nil, nil, err
		}

		fields[sanitizedFieldName] = &graphql.Field{
			Type: fieldType,
		}

		inputFields[sanitizedFieldName] = &graphql.InputObjectFieldConfig{
			Type: inputFieldType,
		}
	}

	return fields, inputFields, nil
}

func (g *Gateway) convertSwaggerTypeToGraphQL(schema spec.Schema, typePrefix string, fieldPath []string, processingTypes map[string]bool) (graphql.Output, graphql.Input, error) {
	if len(schema.Type) == 0 {
		// Handle $ref types
		if schema.Ref.GetURL() != nil {
			refKey := schema.Ref.String()

			// Remove the leading '#/definitions/' from the ref string
			refKey = strings.TrimPrefix(refKey, "#/definitions/")

			// Check if type is already being processed
			if processingTypes[refKey] {
				// Return existing type to prevent infinite recursion
				if existingType, exists := g.typesCache[refKey]; exists {
					existingInputType := g.inputTypesCache[refKey]
					return existingType, existingInputType, nil
				}
				// Return placeholder types to prevent recursion
				return graphql.String, graphql.String, nil
			}

			if refDef, ok := g.definitions[refKey]; ok {
				// Mark as processing
				processingTypes[refKey] = true
				defer delete(processingTypes, refKey)

				fieldType, inputFieldType, err := g.convertSwaggerTypeToGraphQL(refDef, refKey, fieldPath, processingTypes)
				if err != nil {
					return nil, nil, err
				}

				// Store the types
				if objType, ok := fieldType.(*graphql.Object); ok {
					g.typesCache[refKey] = objType
				}
				if inputObjType, ok := inputFieldType.(*graphql.InputObject); ok {
					g.inputTypesCache[refKey] = inputObjType
				}

				return fieldType, inputFieldType, nil
			} else {
				// Definition not found, return string
				return graphql.String, graphql.String, nil
			}
		}
		return graphql.String, graphql.String, nil
	}

	switch schema.Type[0] {
	case "string":
		return graphql.String, graphql.String, nil
	case "integer":
		return graphql.Int, graphql.Int, nil
	case "number":
		return graphql.Float, graphql.Float, nil
	case "boolean":
		return graphql.Boolean, graphql.Boolean, nil
	case "array":
		if schema.Items != nil && schema.Items.Schema != nil {
			itemType, inputItemType, err := g.convertSwaggerTypeToGraphQL(*schema.Items.Schema, typePrefix, fieldPath, processingTypes)
			if err != nil {
				return nil, nil, err
			}
			return graphql.NewList(itemType), graphql.NewList(inputItemType), nil
		}
		return graphql.NewList(graphql.String), graphql.NewList(graphql.String), nil
	case "object":
		return g.handleObjectFieldSpecType(schema, typePrefix, fieldPath, processingTypes)
	default:
		// Handle unexpected types or additional properties
		return graphql.String, graphql.String, nil
	}
}

func (g *Gateway) handleObjectFieldSpecType(fieldSpec spec.Schema, typePrefix string, fieldPath []string, processingTypes map[string]bool) (graphql.Output, graphql.Input, error) {
	if len(fieldSpec.Properties) > 0 {
		typeName := g.generateTypeName(typePrefix, fieldPath)

		// Check if type already generated
		if existingType, exists := g.typesCache[typeName]; exists {
			return existingType, g.inputTypesCache[typeName], nil
		}

		// Store placeholder to prevent recursion
		g.typesCache[typeName] = nil
		g.inputTypesCache[typeName] = nil

		nestedFields, nestedInputFields, err := g.generateGraphQLFields(&fieldSpec, typeName, fieldPath, processingTypes)
		if err != nil {
			return nil, nil, err
		}

		newType := graphql.NewObject(graphql.ObjectConfig{
			Name:   sanitizeFieldName(typeName),
			Fields: nestedFields,
		})

		newInputType := graphql.NewInputObject(graphql.InputObjectConfig{
			Name:   sanitizeFieldName(typeName) + "Input",
			Fields: nestedInputFields,
		})

		// Store the generated types
		g.typesCache[typeName] = newType
		g.inputTypesCache[typeName] = newInputType

		return newType, newInputType, nil
	} else if fieldSpec.AdditionalProperties != nil && fieldSpec.AdditionalProperties.Schema != nil {
		// Hagndle map types
		if len(fieldSpec.AdditionalProperties.Schema.Type) == 1 && fieldSpec.AdditionalProperties.Schema.Type[0] == "string" {
			// This is a map[string]string
			return stringMapScalar, stringMapScalar, nil
		}
	}

	// It's an empty object
	return graphql.String, graphql.String, nil
}

func (g *Gateway) generateTypeName(typePrefix string, fieldPath []string) string {
	name := typePrefix + strings.Join(fieldPath, "")
	return name
}

// io.openmfp.core.v1alpha1.Account

// getGroupVersionKind retrieves the GroupVersionKind for a given resourceKey and its OpenAPI schema.
// It first checks for the 'x-kubernetes-group-version-kind' extension and uses it if available.
// If not, it falls back to parsing the resourceKey.
func (g *Gateway) getGroupVersionKind(resourceKey string) (*schema.GroupVersionKind, error) {
	// First, check if 'x-kubernetes-group-version-kind' extension is present
	resourceSpec, ok := g.definitions[resourceKey]
	if !ok || resourceSpec.Extensions == nil {
		return nil, errors.New("no resource extensions")
	}
	xkGvk, ok := resourceSpec.Extensions[common.GVKExtensionKey]
	if !ok {
		return nil, errors.New("x-kubernetes-group-version-kind extension not found")
	}
	// xkGvk should be an array of maps
	if gvkList, ok := xkGvk.([]interface{}); ok && len(gvkList) > 0 {
		// Use the first item in the list
		if gvkMap, ok := gvkList[0].(map[string]interface{}); ok {
			group, _ := gvkMap["group"].(string)
			version, _ := gvkMap["version"].(string)
			kind, _ := gvkMap["kind"].(string)

			// Sanitize the group and kind names
			return &schema.GroupVersionKind{
				Group:   g.resolver.SanitizeGroupName(group),
				Version: version,
				Kind:    kind,
			}, nil
		}
	}

	return nil, errors.New("failed to parse x-kubernetes-group-version-kind extension")
}

func (g *Gateway) storeCategory(
	resourceKey string,
	gvk *schema.GroupVersionKind,
	resourceScope apiextensionsv1.ResourceScope,
) error {
	resourceSpec, ok := g.definitions[resourceKey]
	if !ok || resourceSpec.Extensions == nil {
		return errors.New("no resource extensions")
	}
	categoriesRaw, ok := resourceSpec.Extensions[common.CategoriesExtensionKey]
	if !ok {
		return fmt.Errorf("%s extension not found", common.CategoriesExtensionKey)
	}

	categoriesRawArray, ok := categoriesRaw.([]interface{})
	if !ok {
		return fmt.Errorf("%s extension is not an array", common.CategoriesExtensionKey)
	}

	categories := make([]string, len(categoriesRawArray))
	for i, v := range categoriesRawArray {
		if str, ok := v.(string); ok {
			categories[i] = str
		} else {
			return fmt.Errorf("failed to convert %d to string", v)
		}
	}

	for _, category := range categories {
		g.typeByCategory[category] = append(g.typeByCategory[category], resolver.TypeByCategory{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
			Scope:   string(resourceScope),
		})
	}

	return nil
}

func (g *Gateway) getScope(resourceURI string) (apiextensionsv1.ResourceScope, error) {
	resourceSpec, ok := g.definitions[resourceURI]
	if !ok {
		return "", errors.New("no resource found")
	}
	if resourceSpec.Extensions == nil {
		return "", errors.New("no resource extensions")
	}
	scopeRaw, ok := resourceSpec.Extensions[common.ScopeExtensionKey]
	if !ok {
		g.log.Debug().Str("resource", resourceURI).Msg("scope extension not found")
		return "", nil
	}

	scope, ok := scopeRaw.(string)
	if !ok {
		return "", errors.New("failed to parse scope extension as a string")
	}

	return apiextensionsv1.ResourceScope(scope), nil
}

func sanitizeFieldName(name string) string {
	// Replace any invalid characters with '_'
	name = regexp.MustCompile(`[^_a-zA-Z0-9]`).ReplaceAllString(name, "_")

	// If the name doesn't start with a letter or underscore, prepend '_'
	if !regexp.MustCompile(`^[_a-zA-Z]`).MatchString(name) {
		name = "_" + name
	}

	return name
}

func addErrorFieldToGraphqlObject(obj *graphql.Object) *graphql.Object {
	obj.AddFieldConfig("error", &graphql.Field{
		Type: graphql.String,
	})
	return obj
}

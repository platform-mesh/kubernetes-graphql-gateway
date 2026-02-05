package schema

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gobuffalo/flect"
	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/apis"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema/types"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// watchEventTypeEnum defines constant event types for subscriptions
var watchEventTypeEnum = graphql.NewEnum(graphql.EnumConfig{
	Name:        "WatchEventType",
	Description: "Event type for resource change notifications",
	Values: graphql.EnumValueConfigMap{
		"ADDED":    &graphql.EnumValueConfig{Value: resolver.EventTypeAdded},
		"MODIFIED": &graphql.EnumValueConfig{Value: resolver.EventTypeModified},
		"DELETED":  &graphql.EnumValueConfig{Value: resolver.EventTypeDeleted},
	},
})

type Provider interface {
	GetSchema() *graphql.Schema
}

type Gateway struct {
	resolver      resolver.Provider
	graphqlSchema graphql.Schema
	definitions   map[string]*spec.Schema

	// Type management via types package
	typeRegistry  *types.Registry
	typeConverter *types.Converter

	// categoryRegistry stores resources by category for typeByCategory query
	typeByCategory map[string][]resolver.TypeByCategory
}

func New(definitions map[string]*spec.Schema, resolverProvider resolver.Provider) (*Gateway, error) {
	registry := types.NewRegistry(resolverProvider.SanitizeGroupName)

	g := &Gateway{
		resolver:       resolverProvider,
		definitions:    definitions,
		typeRegistry:   registry,
		typeConverter:  types.NewConverter(registry, definitions),
		typeByCategory: make(map[string][]resolver.TypeByCategory),
	}

	err := g.generateGraphqlSchema(context.TODO())

	return g, err
}

func (g *Gateway) GetSchema() *graphql.Schema {
	return &g.graphqlSchema
}

func (g *Gateway) generateGraphqlSchema(ctx context.Context) error {
	logger := log.FromContext(ctx)
	rootQueryFields := graphql.Fields{}
	rootMutationFields := graphql.Fields{}
	rootSubscriptionFields := graphql.Fields{}

	groups, err := g.getDefinitionsByGroup(g.definitions)
	if err != nil {
		return err
	}

	for group, groupedResources := range groups {
		g.processGroupedResources(
			ctx,
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
			Name:   "Query",
			Fields: rootQueryFields,
		}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{
			Name:   "Mutation",
			Fields: rootMutationFields,
		}),
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name:   "Subscription",
			Fields: rootSubscriptionFields,
		}),
	})

	if err != nil {
		logger.Error(err, "Error creating GraphQL schema")
		return err
	}

	g.graphqlSchema = newSchema

	return nil
}

func (g *Gateway) isRootGroup(group string) bool {
	return group == ""
}

func (g *Gateway) processGroupedResources(
	ctx context.Context,
	group string,
	groupedResources map[string]*spec.Schema,
	rootQueryFields,
	rootMutationFields,
	rootSubscriptionFields graphql.Fields,
) {
	logger := log.FromContext(ctx)
	isRoot := g.isRootGroup(group)
	sanitizedGroup := ""
	if !isRoot {
		sanitizedGroup = g.resolver.SanitizeGroupName(group)
	}

	var queryGroupType, mutationGroupType *graphql.Object
	if !isRoot {
		queryGroupType = graphql.NewObject(graphql.ObjectConfig{
			Name:   flect.Pascalize(sanitizedGroup) + "Query",
			Fields: graphql.Fields{},
		})

		mutationGroupType = graphql.NewObject(graphql.ObjectConfig{
			Name:   flect.Pascalize(sanitizedGroup) + "Mutation",
			Fields: graphql.Fields{},
		})
	}

	versions := map[string]map[string]*spec.Schema{}
	for resourceKey, resourceScheme := range groupedResources {
		gvk, err := g.getGroupVersionKind(resourceKey)
		if err != nil {
			logger.V(4).WithValues("resourceKey", resourceKey).Info("Failed to get GVK while grouping by version")
			continue
		}
		if _, ok := versions[gvk.Version]; !ok {
			versions[gvk.Version] = map[string]*spec.Schema{}
		}
		versions[gvk.Version][resourceKey] = resourceScheme
	}

	for versionStr, resources := range versions {
		// Version objects
		queryVersionType := graphql.NewObject(graphql.ObjectConfig{
			Name:   flect.Pascalize(sanitizedGroup+"_"+versionStr) + "Query",
			Fields: graphql.Fields{},
		})
		mutationVersionType := graphql.NewObject(graphql.ObjectConfig{
			Name:   flect.Pascalize(sanitizedGroup+"_"+versionStr) + "Mutation",
			Fields: graphql.Fields{},
		})

		// Add all resources into the version objects
		for resourceKey, resourceScheme := range resources {
			g.processSingleResource(
				ctx,
				resourceKey,
				resourceScheme,
				queryVersionType,
				mutationVersionType,
				rootSubscriptionFields,
			)
		}

		if len(queryVersionType.Fields()) > 0 {
			if isRoot {
				rootQueryFields[versionStr] = &graphql.Field{
					Type:    queryVersionType,
					Resolve: g.resolver.CommonResolver(),
				}
			} else {
				queryGroupType.AddFieldConfig(versionStr, &graphql.Field{
					Type:    queryVersionType,
					Resolve: g.resolver.CommonResolver(),
				})
			}
		}
		if len(mutationVersionType.Fields()) > 0 {
			if isRoot {
				rootMutationFields[versionStr] = &graphql.Field{
					Type:    mutationVersionType,
					Resolve: g.resolver.CommonResolver(),
				}
			} else {
				mutationGroupType.AddFieldConfig(versionStr, &graphql.Field{
					Type:    mutationVersionType,
					Resolve: g.resolver.CommonResolver(),
				})
			}
		}
	}

	if !isRoot {
		if len(queryGroupType.Fields()) > 0 {
			rootQueryFields[sanitizedGroup] = &graphql.Field{
				Type:    queryGroupType,
				Resolve: g.resolver.CommonResolver(),
			}
		}
		if len(mutationGroupType.Fields()) > 0 {
			rootMutationFields[sanitizedGroup] = &graphql.Field{
				Type:    mutationGroupType,
				Resolve: g.resolver.CommonResolver(),
			}
		}
	}
}

func (g *Gateway) processSingleResource(
	ctx context.Context,
	resourceKey string,
	resourceScheme *spec.Schema,
	queryGroupType, mutationGroupType *graphql.Object,
	rootSubscriptionFields graphql.Fields,
) {
	logger := log.FromContext(ctx)
	gvk, err := g.getGroupVersionKind(resourceKey)
	if err != nil {
		logger.Error(err, "Error getting GVK", "resource", resourceKey)
		return
	}

	if strings.HasSuffix(gvk.Kind, "List") {
		// Skip List resources
		return
	}

	resourceScope, err := g.getScope(resourceKey)
	if err != nil {
		logger.WithValues("resource", resourceKey).Error(err, "Error getting resourceScope")
		return
	}

	err = g.storeCategory(resourceKey, gvk, resourceScope)
	if err != nil {
		// Not all resources have categories - this is expected and not an error
		// Resources without categories won't appear in category-based queries
		logger.V(4).WithValues("resource", resourceKey).Info("Resource has no categories", "reason", err.Error())
	}

	singular, plural := g.getNames(gvk)
	uniqueTypeName := g.getUniqueTypeName(gvk)

	// Generate both fields and inputFields
	fields, inputFields, err := g.generateGraphQLFields(resourceScheme, uniqueTypeName, []string{}, make(map[string]bool))
	if err != nil {
		logger.WithValues("resource", singular).Error(err, "Error generating fields")
		return
	}

	if len(fields) == 0 {
		logger.V(4).WithValues("resource", singular).Info("No fields found")
		return
	}

	resourceType := graphql.NewObject(graphql.ObjectConfig{
		Name:   uniqueTypeName,
		Fields: fields,
	})

	resourceInputType := graphql.NewInputObject(graphql.InputObjectConfig{
		Name:   uniqueTypeName + "Input",
		Fields: inputFields,
	})

	listArgsBuilder := resolver.NewFieldConfigArguments().
		WithLabelSelector().
		WithSortBy().
		WithLimit().
		WithContinue()

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

	listWrapperType := graphql.NewObject(graphql.ObjectConfig{
		Name: uniqueTypeName + "List",
		Fields: graphql.Fields{
			"resourceVersion":    &graphql.Field{Type: graphql.String},
			"items":              &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(resourceType)))},
			"continue":           &graphql.Field{Type: graphql.String},
			"remainingItemCount": &graphql.Field{Type: graphql.Int},
		},
	})

	queryGroupType.AddFieldConfig(plural, &graphql.Field{
		Type:    graphql.NewNonNull(listWrapperType),
		Args:    listArgs,
		Resolve: g.resolver.ListItems(ctx, *gvk, resourceScope),
	})

	queryGroupType.AddFieldConfig(singular, &graphql.Field{
		Type:    graphql.NewNonNull(resourceType),
		Args:    itemArgs,
		Resolve: g.resolver.GetItem(ctx, *gvk, resourceScope),
	})

	queryGroupType.AddFieldConfig(singular+"Yaml", &graphql.Field{
		Type:    graphql.NewNonNull(graphql.String),
		Args:    itemArgs,
		Resolve: g.resolver.GetItemAsYAML(ctx, *gvk, resourceScope),
	})

	// Mutation definitions
	mutationGroupType.AddFieldConfig("create"+singular, &graphql.Field{
		Type:    resourceType,
		Args:    creationMutationArgs,
		Resolve: g.resolver.CreateItem(ctx, *gvk, resourceScope),
	})

	mutationGroupType.AddFieldConfig("update"+singular, &graphql.Field{
		Type:    resourceType,
		Args:    creationMutationArgsBuilder.WithName().Complete(),
		Resolve: g.resolver.UpdateItem(ctx, *gvk, resourceScope),
	})

	mutationGroupType.AddFieldConfig("delete"+singular, &graphql.Field{
		Type:    graphql.Boolean,
		Args:    itemArgsBuilder.WithDryRun().Complete(),
		Resolve: g.resolver.DeleteItem(ctx, *gvk, resourceScope),
	})

	// Define an event envelope type for subscriptions
	eventType := graphql.NewObject(graphql.ObjectConfig{
		Name: uniqueTypeName + "Event",
		Fields: graphql.Fields{
			"type":   &graphql.Field{Type: graphql.NewNonNull(watchEventTypeEnum)},
			"object": &graphql.Field{Type: resourceType},
		},
	})

	var subscriptionSingular string
	sanitizedGroup := ""
	if !g.isRootGroup(gvk.Group) {
		sanitizedGroup = g.resolver.SanitizeGroupName(gvk.Group)
	}

	if sanitizedGroup == "" {
		subscriptionSingular = strings.ToLower(fmt.Sprintf("%s_%s", gvk.Version, singular))
	} else {
		subscriptionSingular = strings.ToLower(fmt.Sprintf("%s_%s_%s", sanitizedGroup, gvk.Version, singular))
	}

	rootSubscriptionFields[subscriptionSingular] = &graphql.Field{
		Type: eventType,
		Args: itemArgsBuilder.
			WithSubscribeToAll().
			WithResourceVersion().
			Complete(),
		Resolve:     resolver.CreateSubscriptionResolver(true),
		Subscribe:   g.resolver.SubscribeItem(ctx, *gvk, resourceScope),
		Description: fmt.Sprintf("Subscribe to changes of %s", singular),
	}

	var subscriptionPlural string
	if sanitizedGroup == "" {
		subscriptionPlural = strings.ToLower(fmt.Sprintf("%s_%s", gvk.Version, plural))
	} else {
		subscriptionPlural = strings.ToLower(fmt.Sprintf("%s_%s_%s", sanitizedGroup, gvk.Version, plural))
	}
	rootSubscriptionFields[subscriptionPlural] = &graphql.Field{
		Type: eventType,
		Args: listArgsBuilder.
			WithSubscribeToAll().
			WithResourceVersion().
			Complete(),
		Resolve:     resolver.CreateSubscriptionResolver(false),
		Subscribe:   g.resolver.SubscribeItems(ctx, *gvk, resourceScope),
		Description: fmt.Sprintf("Subscribe to changes of %s", plural),
	}
}

func (g *Gateway) getUniqueTypeName(gvk *schema.GroupVersionKind) string {
	return g.typeRegistry.GetUniqueTypeName(gvk)
}

func (g *Gateway) getNames(gvk *schema.GroupVersionKind) (singular string, plural string) {
	kind := gvk.Kind
	singular = kind
	plural = flect.Pluralize(singular)

	return singular, plural
}

func (g *Gateway) getDefinitionsByGroup(filteredDefinitions map[string]*spec.Schema) (map[string]map[string]*spec.Schema, error) {
	groups := map[string]map[string]*spec.Schema{}
	for key, definition := range filteredDefinitions {
		gvk, err := g.getGroupVersionKind(key)
		if err != nil {
			// Skip definitions without valid GVK - these are typically helper types
			// (like ListMeta, ObjectMeta) or sub-resources that don't represent
			// top-level Kubernetes resources
			continue
		}

		// Skip definitions with empty Kind - these are invalid/corrupted entries
		if gvk.Kind == "" {
			continue
		}

		// Skip definitions without scope extension - these are helper types
		// (like DeleteOptions, APIVersions) that aren't queryable resources
		if _, err := g.getScope(key); err != nil {
			continue
		}

		if _, ok := groups[gvk.Group]; !ok {
			groups[gvk.Group] = map[string]*spec.Schema{}
		}

		groups[gvk.Group][key] = definition
	}

	return groups, nil
}

func (g *Gateway) generateGraphQLFields(resourceScheme *spec.Schema, typePrefix string, fieldPath []string, processingTypes map[string]bool) (graphql.Fields, graphql.InputObjectConfigFieldMap, error) {
	return g.typeConverter.ConvertFields(resourceScheme, typePrefix, fieldPath, processingTypes)
}

// parseGVKExtension parses the x-kubernetes-group-version-kind extension from a resource schema.
func parseGVKExtension(extensions map[string]any, resourceKey string) (*schema.GroupVersionKind, error) {
	xkGvk, ok := extensions[apis.GVKExtensionKey]
	if !ok {
		return nil, errors.New("x-kubernetes-group-version-kind extension not found")
	}

	gvkList, ok := xkGvk.([]any)
	if !ok || len(gvkList) == 0 {
		return nil, errors.New("invalid GVK extension format")
	}

	gvkMap, ok := gvkList[0].(map[string]any)
	if !ok {
		return nil, errors.New("invalid GVK map format")
	}

	group, _ := gvkMap["group"].(string)
	versionStr, _ := gvkMap["version"].(string)
	kind, _ := gvkMap["kind"].(string)

	if kind == "" {
		return nil, fmt.Errorf("kind cannot be empty for resource %s", resourceKey)
	}

	return &schema.GroupVersionKind{
		Group:   group,
		Version: versionStr,
		Kind:    kind,
	}, nil
}

// getGroupVersionKindFromDefinitions retrieves the GroupVersionKind for a given resourceKey from a definitions map.
// This is a standalone function that doesn't require a Gateway receiver.
func getGroupVersionKindFromDefinitions(resourceKey string, definitions map[string]*spec.Schema) (*schema.GroupVersionKind, error) {
	resourceSpec, ok := definitions[resourceKey]
	if !ok || resourceSpec.Extensions == nil {
		return nil, errors.New("no resource extensions")
	}

	return parseGVKExtension(resourceSpec.Extensions, resourceKey)
}

// getGroupVersionKind retrieves the GroupVersionKind for a given resourceKey and its OpenAPI schema.
// It uses the standalone helper but applies group name sanitization for GraphQL compatibility.
func (g *Gateway) getGroupVersionKind(resourceKey string) (*schema.GroupVersionKind, error) {
	gvk, err := getGroupVersionKindFromDefinitions(resourceKey, g.definitions)
	if err != nil {
		return nil, err
	}

	// Sanitize the group name for GraphQL compatibility
	gvk.Group = g.resolver.SanitizeGroupName(gvk.Group)
	return gvk, nil
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
	categoriesRaw, ok := resourceSpec.Extensions[apis.CategoriesExtensionKey]
	if !ok {
		return fmt.Errorf("%s extension not found", apis.CategoriesExtensionKey)
	}

	categoriesRawArray, ok := categoriesRaw.([]any)
	if !ok {
		return fmt.Errorf("%s extension is not an array", apis.CategoriesExtensionKey)
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
	scopeRaw, ok := resourceSpec.Extensions[apis.ScopeExtensionKey]
	if !ok {
		return "", errors.New("scope extension not found")
	}

	scope, ok := scopeRaw.(string)
	if !ok {
		return "", errors.New("failed to parse scope extension as a string")
	}

	return apiextensionsv1.ResourceScope(scope), nil
}

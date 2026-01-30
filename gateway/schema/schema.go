package schema

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gobuffalo/flect"
	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/apis"
	common "github.com/platform-mesh/kubernetes-graphql-gateway/apis"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/kube-openapi/pkg/validation/spec"
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
	resolver           resolver.Provider
	graphqlSchema      graphql.Schema
	definitions        map[string]*spec.Schema
	typesCache         map[string]*graphql.Object
	inputTypesCache    map[string]*graphql.InputObject
	enhancedTypesCache map[string]*graphql.Object // Cache for enhanced *Ref types
	// Prevents naming conflict in case of the same Kind name in different groups/versions
	typeNameRegistry map[string]string // map[Kind]GroupVersion

	// categoryRegistry stores resources by category for typeByCategory query
	typeByCategory map[string][]resolver.TypeByCategory
}

func New(definitions map[string]*spec.Schema, resolverProvider resolver.Provider) (*Gateway, error) {
	g := &Gateway{
		resolver:           resolverProvider,
		definitions:        definitions,
		typesCache:         make(map[string]*graphql.Object),
		inputTypesCache:    make(map[string]*graphql.InputObject),
		enhancedTypesCache: make(map[string]*graphql.Object),
		typeNameRegistry:   make(map[string]string),
		typeByCategory:     make(map[string][]resolver.TypeByCategory),
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

// highestSemverVersion finds the highest semantic version among a group of resources with the same Kind.
// It extracts the version from each GroupVersionKind string and compares them using Kubernetes version priority.
// Returns the full GroupVersionKind string of the resource with the highest version.
func highestSemverVersion(currentKind string, otherVersions map[string]*spec.Schema, definitions map[string]*spec.Schema) string {
	highestKey := currentKind
	highestVersion := ""

	// Extract version from current kind
	if gvk, err := getGroupVersionKindFromDefinitions(currentKind, definitions); err == nil {
		highestVersion = gvk.Version
	}

	// Compare with other versions
	for versionKey := range otherVersions {
		gvk, err := getGroupVersionKindFromDefinitions(versionKey, definitions)
		if err != nil {
			continue
		}

		// Compare versions using Kubernetes version comparison
		// CompareKubeAwareVersionStrings returns positive if v1 > v2, 0 if equal, negative if v1 < v2
		if version.CompareKubeAwareVersionStrings(gvk.Version, highestVersion) > 0 {
			highestVersion = gvk.Version
			highestKey = versionKey
		}
	}

	return highestKey
}

// hasAnotherVersion checks if there are other versions of the same resource (same Group and Kind, different Version).
// It returns true if other versions exist, and a map of all other versions found.
func hasAnotherVersion(groupVersionKind string, allKinds map[string]*spec.Schema, definitions map[string]*spec.Schema) (bool, map[string]*spec.Schema) {
	// Get the GVK for the current resource
	currentGVK, err := getGroupVersionKindFromDefinitions(groupVersionKind, definitions)
	if err != nil {
		// If we can't parse the current GVK, we can't determine if there are other versions
		return false, nil
	}

	otherVersions := map[string]*spec.Schema{}
	hasOtherVersion := false

	// Check all other resources to find ones with the same Group and Kind but different Version
	for otherResourceKey, otherSchema := range allKinds {
		// Skip the current resource
		if otherResourceKey == groupVersionKind {
			continue
		}

		// Get the GVK for the other resource
		otherGVK, err := getGroupVersionKindFromDefinitions(otherResourceKey, definitions)
		if err != nil {
			// Skip resources we can't parse
			continue
		}

		// Check if it's the same Group and Kind but different Version
		if otherGVK.Group == currentGVK.Group &&
			otherGVK.Kind == currentGVK.Kind &&
			otherGVK.Version != currentGVK.Version {
			hasOtherVersion = true
			otherVersions[otherResourceKey] = otherSchema
		}
	}

	return hasOtherVersion, otherVersions
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
	kind := gvk.Kind
	// Check if the kind name has already been used for a different group/version
	if existingGroupVersion, exists := g.typeNameRegistry[kind]; exists {
		if existingGroupVersion != gvk.GroupVersion().String() {
			// Conflict detected, append group and version to the kind for uniqueness
			sanitizedGroup := ""
			if !g.isRootGroup(gvk.Group) {
				sanitizedGroup = g.resolver.SanitizeGroupName(gvk.Group)
			}
			return flect.Pascalize(sanitizedGroup+"_"+gvk.Version) + kind
		}
	} else {
		// No conflict, register the kind with its group and version
		g.typeNameRegistry[kind] = gvk.GroupVersion().String()
	}

	return kind
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

	// Add relation fields for any *Ref fields in this schema
	g.addRelationFields(fields, resourceScheme.Properties)

	return fields, inputFields, nil
}

func (g *Gateway) convertSwaggerTypeToGraphQL(schema spec.Schema, typePrefix string, fieldPath []string, processingTypes map[string]bool) (graphql.Output, graphql.Input, error) {
	if len(schema.Type) == 0 {
		// Handle $ref types
		if len(schema.AllOf) == 0 {
			return graphql.String, graphql.String, nil
		}

		refKey := schema.AllOf[0].Ref.String()

		// Check if type is already being processed
		if processingTypes[refKey] {
			// Return existing type to prevent infinite recursion
			if existingType, exists := g.typesCache[refKey]; exists && existingType != nil {
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

			fieldType, inputFieldType, err := g.convertSwaggerTypeToGraphQL(*refDef, refKey, fieldPath, processingTypes)
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
		if existingType, exists := g.typesCache[typeName]; exists && existingType != nil {
			return existingType, g.inputTypesCache[typeName], nil
		}

		// If type is being processed (nil in cache), return placeholder to prevent recursion
		if _, exists := g.typesCache[typeName]; exists {
			return graphql.String, graphql.String, nil
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

	// It's an empty object, serialize as JSON string
	return jsonStringScalar, jsonStringScalar, nil
}

func (g *Gateway) generateTypeName(typePrefix string, fieldPath []string) string {
	name := typePrefix + strings.Join(fieldPath, "")
	return name
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
	categoriesRaw, ok := resourceSpec.Extensions[common.CategoriesExtensionKey]
	if !ok {
		return fmt.Errorf("%s extension not found", common.CategoriesExtensionKey)
	}

	categoriesRawArray, ok := categoriesRaw.([]any)
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
		return "", errors.New("scope extension not found")
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

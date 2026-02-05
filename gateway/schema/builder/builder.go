package builder

import (
	"context"

	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema/extensions"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema/fields"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema/types"

	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Builder orchestrates the GraphQL schema construction process.
type Builder struct {
	definitions map[string]*spec.Schema
	resolver    resolver.Provider

	// Components
	typeRegistry          *types.Registry
	typeConverter         *types.Converter
	grouper               *Grouper
	categoryManager       *extensions.CategoryManager
	queryGenerator        *fields.QueryGenerator
	mutationGenerator     *fields.MutationGenerator
	subscriptionGenerator *fields.SubscriptionGenerator
	customQueryGenerator  *extensions.CustomQueryGenerator
}

// New creates a new schema builder with all required components.
func New(definitions map[string]*spec.Schema, resolverProvider resolver.Provider) *Builder {
	registry := types.NewRegistry(resolverProvider.SanitizeGroupName)
	categoryManager := extensions.NewCategoryManager(definitions)

	return &Builder{
		definitions:           definitions,
		resolver:              resolverProvider,
		typeRegistry:          registry,
		typeConverter:         types.NewConverter(registry),
		grouper:               NewGrouper(definitions, resolverProvider),
		categoryManager:       categoryManager,
		queryGenerator:        fields.NewQueryGenerator(resolverProvider),
		mutationGenerator:     fields.NewMutationGenerator(resolverProvider),
		subscriptionGenerator: fields.NewSubscriptionGenerator(resolverProvider),
		customQueryGenerator:  extensions.NewCustomQueryGenerator(resolverProvider, categoryManager),
	}
}

// Build constructs the complete GraphQL schema.
func (b *Builder) Build(ctx context.Context) (*graphql.Schema, error) {
	logger := log.FromContext(ctx)

	rootQueryFields := graphql.Fields{}
	rootMutationFields := graphql.Fields{}
	rootSubscriptionFields := graphql.Fields{}

	groups, err := b.grouper.GroupByAPIGroup()
	if err != nil {
		return nil, err
	}

	for group, groupedResources := range groups {
		b.processGroup(
			ctx,
			group,
			groupedResources,
			rootQueryFields,
			rootMutationFields,
			rootSubscriptionFields,
		)
	}

	// Add custom queries
	b.customQueryGenerator.AddTypeByCategoryQuery(rootQueryFields)

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
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
		return nil, err
	}

	return &schema, nil
}

// processGroup processes all resources in an API group.
func (b *Builder) processGroup(
	ctx context.Context,
	group string,
	groupedResources map[string]*spec.Schema,
	rootQueryFields,
	rootMutationFields,
	rootSubscriptionFields graphql.Fields,
) {
	logger := log.FromContext(ctx)
	isRoot := b.grouper.IsRootGroup(group)
	sanitizedGroup := ""
	if !isRoot {
		sanitizedGroup = b.resolver.SanitizeGroupName(group)
	}

	var queryGroupType, mutationGroupType *graphql.Object
	if !isRoot {
		queryGroupType = b.grouper.CreateGroupType(sanitizedGroup, "Query")
		mutationGroupType = b.grouper.CreateGroupType(sanitizedGroup, "Mutation")
	}

	versions := b.grouper.GroupByVersion(groupedResources)

	for versionStr, resources := range versions {
		queryVersionType := b.grouper.CreateVersionType(sanitizedGroup, versionStr, "Query")
		mutationVersionType := b.grouper.CreateVersionType(sanitizedGroup, versionStr, "Mutation")

		for resourceKey, resourceScheme := range resources {
			b.processResource(
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
					Resolve: b.resolver.CommonResolver(),
				}
			} else {
				queryGroupType.AddFieldConfig(versionStr, &graphql.Field{
					Type:    queryVersionType,
					Resolve: b.resolver.CommonResolver(),
				})
			}
		}
		if len(mutationVersionType.Fields()) > 0 {
			if isRoot {
				rootMutationFields[versionStr] = &graphql.Field{
					Type:    mutationVersionType,
					Resolve: b.resolver.CommonResolver(),
				}
			} else {
				mutationGroupType.AddFieldConfig(versionStr, &graphql.Field{
					Type:    mutationVersionType,
					Resolve: b.resolver.CommonResolver(),
				})
			}
		}
	}

	if !isRoot {
		if len(queryGroupType.Fields()) > 0 {
			rootQueryFields[sanitizedGroup] = &graphql.Field{
				Type:    queryGroupType,
				Resolve: b.resolver.CommonResolver(),
			}
		}
		if len(mutationGroupType.Fields()) > 0 {
			rootMutationFields[sanitizedGroup] = &graphql.Field{
				Type:    mutationGroupType,
				Resolve: b.resolver.CommonResolver(),
			}
		}
	}

	logger.V(4).Info("Processed group", "group", group, "resourceCount", len(groupedResources))
}

// processResource processes a single Kubernetes resource.
func (b *Builder) processResource(
	ctx context.Context,
	resourceKey string,
	resourceScheme *spec.Schema,
	queryGroupType, mutationGroupType *graphql.Object,
	rootSubscriptionFields graphql.Fields,
) {
	logger := log.FromContext(ctx)

	gvk, err := b.grouper.GetGroupVersionKind(resourceKey)
	if err != nil {
		logger.Error(err, "Error getting GVK", "resource", resourceKey)
		return
	}

	if b.grouper.ShouldSkipKind(gvk.Kind) {
		return
	}

	resourceScope, err := b.grouper.GetScope(resourceKey)
	if err != nil {
		logger.WithValues("resource", resourceKey).Error(err, "Error getting resourceScope")
		return
	}

	// Store category (optional, not all resources have categories)
	if err := b.categoryManager.Store(resourceKey, gvk, resourceScope); err != nil {
		logger.V(4).WithValues("resource", resourceKey).Info("Resource has no categories", "reason", err.Error())
	}

	singular, plural := b.grouper.GetNames(gvk)
	uniqueTypeName := b.typeRegistry.GetUniqueTypeName(gvk)

	// Generate GraphQL fields
	gqlFields, inputFields, err := b.typeConverter.ConvertFields(resourceScheme, b.definitions, uniqueTypeName)
	if err != nil {
		logger.WithValues("resource", singular).Error(err, "Error generating fields")
		return
	}

	if len(gqlFields) == 0 {
		logger.V(4).WithValues("resource", singular).Info("No fields found")
		return
	}

	resourceType := graphql.NewObject(graphql.ObjectConfig{
		Name:   uniqueTypeName,
		Fields: gqlFields,
	})

	resourceInputType := graphql.NewInputObject(graphql.InputObjectConfig{
		Name:   uniqueTypeName + "Input",
		Fields: inputFields,
	})

	sanitizedGroup := ""
	if !b.grouper.IsRootGroup(gvk.Group) {
		sanitizedGroup = b.resolver.SanitizeGroupName(gvk.Group)
	}

	rc := &fields.ResourceContext{
		GVK:            *gvk,
		Scope:          resourceScope,
		UniqueTypeName: uniqueTypeName,
		ResourceType:   resourceType,
		InputType:      resourceInputType,
		SingularName:   singular,
		PluralName:     plural,
		SanitizedGroup: sanitizedGroup,
	}

	b.queryGenerator.Generate(rc, queryGroupType)
	b.mutationGenerator.Generate(rc, mutationGroupType)
	b.subscriptionGenerator.Generate(rc, rootSubscriptionFields)
}

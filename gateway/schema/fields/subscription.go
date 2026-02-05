package fields

import (
	"fmt"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
)

// WatchEventTypeEnum defines constant event types for subscriptions.
var WatchEventTypeEnum = graphql.NewEnum(graphql.EnumConfig{
	Name:        "WatchEventType",
	Description: "Event type for resource change notifications",
	Values: graphql.EnumValueConfigMap{
		"ADDED":    &graphql.EnumValueConfig{Value: resolver.EventTypeAdded},
		"MODIFIED": &graphql.EnumValueConfig{Value: resolver.EventTypeModified},
		"DELETED":  &graphql.EnumValueConfig{Value: resolver.EventTypeDeleted},
	},
})

// SubscriptionGenerator generates GraphQL subscription fields for a resource.
type SubscriptionGenerator struct {
	resolver resolver.Provider
}

// NewSubscriptionGenerator creates a new subscription field generator.
func NewSubscriptionGenerator(resolver resolver.Provider) *SubscriptionGenerator {
	return &SubscriptionGenerator{resolver: resolver}
}

// Generate adds subscription fields to the target fields map.
// Unlike queries and mutations, subscriptions are added to a flat fields map.
func (g *SubscriptionGenerator) Generate(rc *ResourceContext, target graphql.Fields) {
	listArgsBuilder := resolver.NewFieldConfigArguments().
		WithLabelSelector().
		WithSortBy().
		WithLimit().
		WithContinue()

	itemArgsBuilder := resolver.NewFieldConfigArguments().WithName()

	if rc.IsNamespaceScoped() {
		listArgsBuilder.WithNamespace()
		itemArgsBuilder.WithNamespace()
	}

	// Create event envelope type
	eventType := graphql.NewObject(graphql.ObjectConfig{
		Name: rc.UniqueTypeName + "Event",
		Fields: graphql.Fields{
			"type":   &graphql.Field{Type: graphql.NewNonNull(WatchEventTypeEnum)},
			"object": &graphql.Field{Type: rc.ResourceType},
		},
	})

	// Build subscription field names
	singularName := g.buildSubscriptionName(rc, rc.SingularName)
	pluralName := g.buildSubscriptionName(rc, rc.PluralName)

	// Add singular subscription (watch single resource)
	target[singularName] = &graphql.Field{
		Type: eventType,
		Args: itemArgsBuilder.
			WithSubscribeToAll().
			WithResourceVersion().
			Complete(),
		Resolve:     resolver.CreateSubscriptionResolver(true),
		Subscribe:   g.resolver.SubscribeItem(rc.Ctx, rc.GVK, rc.Scope),
		Description: fmt.Sprintf("Subscribe to changes of %s", rc.SingularName),
	}

	// Add plural subscription (watch multiple resources)
	target[pluralName] = &graphql.Field{
		Type: eventType,
		Args: listArgsBuilder.
			WithSubscribeToAll().
			WithResourceVersion().
			Complete(),
		Resolve:     resolver.CreateSubscriptionResolver(false),
		Subscribe:   g.resolver.SubscribeItems(rc.Ctx, rc.GVK, rc.Scope),
		Description: fmt.Sprintf("Subscribe to changes of %s", rc.PluralName),
	}
}

// buildSubscriptionName creates the subscription field name with group/version prefix.
func (g *SubscriptionGenerator) buildSubscriptionName(rc *ResourceContext, name string) string {
	if rc.SanitizedGroup == "" {
		return strings.ToLower(fmt.Sprintf("%s_%s", rc.GVK.Version, name))
	}
	return strings.ToLower(fmt.Sprintf("%s_%s_%s", rc.SanitizedGroup, rc.GVK.Version, name))
}

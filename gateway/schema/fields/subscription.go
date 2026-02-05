package fields

import (
	"fmt"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
)

var WatchEventTypeEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "WatchEventType",
	Values: graphql.EnumValueConfigMap{
		"ADDED":    &graphql.EnumValueConfig{Value: resolver.EventTypeAdded},
		"MODIFIED": &graphql.EnumValueConfig{Value: resolver.EventTypeModified},
		"DELETED":  &graphql.EnumValueConfig{Value: resolver.EventTypeDeleted},
	},
})

type SubscriptionGenerator struct {
	resolver resolver.Provider
}

func NewSubscriptionGenerator(resolver resolver.Provider) *SubscriptionGenerator {
	return &SubscriptionGenerator{resolver: resolver}
}

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

	eventType := graphql.NewObject(graphql.ObjectConfig{
		Name: rc.UniqueTypeName + "Event",
		Fields: graphql.Fields{
			"type":   &graphql.Field{Type: graphql.NewNonNull(WatchEventTypeEnum)},
			"object": &graphql.Field{Type: rc.ResourceType},
		},
	})

	singularName := g.buildSubscriptionName(rc, rc.SingularName)
	pluralName := g.buildSubscriptionName(rc, rc.PluralName)

	target[singularName] = &graphql.Field{
		Type: eventType,
		Args: itemArgsBuilder.
			WithSubscribeToAll().
			WithResourceVersion().
			Complete(),
		Resolve:     resolver.CreateSubscriptionResolver(true),
		Subscribe:   g.resolver.SubscribeItem(rc.GVK, rc.Scope),
		Description: fmt.Sprintf("Subscribe to changes of %s", rc.SingularName),
	}

	target[pluralName] = &graphql.Field{
		Type: eventType,
		Args: listArgsBuilder.
			WithSubscribeToAll().
			WithResourceVersion().
			Complete(),
		Resolve:     resolver.CreateSubscriptionResolver(false),
		Subscribe:   g.resolver.SubscribeItems(rc.GVK, rc.Scope),
		Description: fmt.Sprintf("Subscribe to changes of %s", rc.PluralName),
	}
}

func (g *SubscriptionGenerator) buildSubscriptionName(rc *ResourceContext, name string) string {
	if rc.SanitizedGroup == "" {
		return strings.ToLower(fmt.Sprintf("%s_%s", rc.GVK.Version, name))
	}
	return strings.ToLower(fmt.Sprintf("%s_%s_%s", rc.SanitizedGroup, rc.GVK.Version, name))
}

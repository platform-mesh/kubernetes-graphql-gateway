package builder

import (
	"errors"
	"strings"

	"github.com/gobuffalo/flect"
	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

type Grouper struct {
	definitions map[string]*spec.Schema
	resolver    resolver.Provider
}

func NewGrouper(definitions map[string]*spec.Schema, resolver resolver.Provider) *Grouper {
	return &Grouper{
		definitions: definitions,
		resolver:    resolver,
	}
}

func (g *Grouper) GroupByAPIGroup() (map[string]map[string]*spec.Schema, error) {
	groups := map[string]map[string]*spec.Schema{}
	for key, definition := range g.definitions {
		gvk, err := g.GetGroupVersionKind(key)
		if err != nil {
			continue
		}

		if gvk.Kind == "" {
			continue
		}

		if _, err := g.GetScope(key); err != nil {
			continue
		}

		if _, ok := groups[gvk.Group]; !ok {
			groups[gvk.Group] = map[string]*spec.Schema{}
		}

		groups[gvk.Group][key] = definition
	}

	return groups, nil
}

func (g *Grouper) GroupByVersion(groupedResources map[string]*spec.Schema) map[string]map[string]*spec.Schema {
	versions := map[string]map[string]*spec.Schema{}
	for resourceKey, resourceScheme := range groupedResources {
		gvk, err := g.GetGroupVersionKind(resourceKey)
		if err != nil {
			continue
		}
		if _, ok := versions[gvk.Version]; !ok {
			versions[gvk.Version] = map[string]*spec.Schema{}
		}
		versions[gvk.Version][resourceKey] = resourceScheme
	}
	return versions
}

func (g *Grouper) GetGroupVersionKind(resourceKey string) (*schema.GroupVersionKind, error) {
	resourceSpec, ok := g.definitions[resourceKey]
	if !ok {
		return nil, errors.New("no resource found")
	}

	gvk, err := apischema.ExtractGVK(resourceSpec)
	if err != nil {
		return nil, err
	}
	if gvk == nil {
		return nil, errors.New("no GVK extension found")
	}

	gvk.Group = g.resolver.SanitizeGroupName(gvk.Group)
	return gvk, nil
}

func (g *Grouper) GetScope(resourceURI string) (apiextensionsv1.ResourceScope, error) {
	resourceSpec, ok := g.definitions[resourceURI]
	if !ok {
		return "", errors.New("no resource found")
	}

	return apischema.ExtractScope(resourceSpec)
}

func (g *Grouper) GetNames(gvk *schema.GroupVersionKind) (singular, plural string) {
	singular = gvk.Kind
	plural = flect.Pluralize(singular)
	return singular, plural
}

func (g *Grouper) IsRootGroup(group string) bool {
	return group == ""
}

func (g *Grouper) ShouldSkipKind(kind string) bool {
	return strings.HasSuffix(kind, "List")
}

func (g *Grouper) CreateGroupType(sanitizedGroup, suffix string) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name:   flect.Pascalize(sanitizedGroup) + suffix,
		Fields: graphql.Fields{},
	})
}

func (g *Grouper) CreateVersionType(sanitizedGroup, version, suffix string) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name:   flect.Pascalize(sanitizedGroup+"_"+version) + suffix,
		Fields: graphql.Fields{},
	})
}

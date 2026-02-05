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

// Grouper handles grouping of OpenAPI definitions by API group and version.
type Grouper struct {
	definitions map[string]*spec.Schema
	resolver    resolver.Provider
}

// NewGrouper creates a new definition grouper.
func NewGrouper(definitions map[string]*spec.Schema, resolver resolver.Provider) *Grouper {
	return &Grouper{
		definitions: definitions,
		resolver:    resolver,
	}
}

// GroupByAPIGroup groups definitions by their Kubernetes API group.
// Returns a map of group name to map of resource key to schema.
func (g *Grouper) GroupByAPIGroup() (map[string]map[string]*spec.Schema, error) {
	groups := map[string]map[string]*spec.Schema{}
	for key, definition := range g.definitions {
		gvk, err := g.GetGroupVersionKind(key)
		if err != nil {
			// Skip definitions without valid GVK - helper types
			continue
		}

		if gvk.Kind == "" {
			continue
		}

		if _, err := g.GetScope(key); err != nil {
			// Skip definitions without scope - helper types
			continue
		}

		if _, ok := groups[gvk.Group]; !ok {
			groups[gvk.Group] = map[string]*spec.Schema{}
		}

		groups[gvk.Group][key] = definition
	}

	return groups, nil
}

// GroupByVersion groups resources within a group by their API version.
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

// GetGroupVersionKind retrieves the GVK for a resource key.
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

	// Convert to runtime GVK and sanitize the group name for GraphQL compatibility
	runtimeGVK := gvk.ToRuntimeGVK()
	runtimeGVK.Group = g.resolver.SanitizeGroupName(runtimeGVK.Group)
	return &runtimeGVK, nil
}

// GetScope retrieves the resource scope (Namespaced or Cluster).
func (g *Grouper) GetScope(resourceURI string) (apiextensionsv1.ResourceScope, error) {
	resourceSpec, ok := g.definitions[resourceURI]
	if !ok {
		return "", errors.New("no resource found")
	}

	return apischema.ExtractScope(resourceSpec)
}

// GetNames returns singular and plural names for a GVK.
func (g *Grouper) GetNames(gvk *schema.GroupVersionKind) (singular, plural string) {
	singular = gvk.Kind
	plural = flect.Pluralize(singular)
	return singular, plural
}

// IsRootGroup returns true if the group is the core API group.
func (g *Grouper) IsRootGroup(group string) bool {
	return group == ""
}

// ShouldSkipKind returns true if the kind should be skipped (e.g., List types).
func (g *Grouper) ShouldSkipKind(kind string) bool {
	return strings.HasSuffix(kind, "List")
}

// CreateGroupType creates a GraphQL object type for a group.
func (g *Grouper) CreateGroupType(sanitizedGroup, suffix string) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name:   flect.Pascalize(sanitizedGroup) + suffix,
		Fields: graphql.Fields{},
	})
}

// CreateVersionType creates a GraphQL object type for a version within a group.
func (g *Grouper) CreateVersionType(sanitizedGroup, version, suffix string) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name:   flect.Pascalize(sanitizedGroup+"_"+version) + suffix,
		Fields: graphql.Fields{},
	})
}

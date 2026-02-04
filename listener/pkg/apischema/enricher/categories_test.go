package enricher_test

import (
	"testing"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema/enricher"
	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

func TestCategoriesEnricher(t *testing.T) {
	podSchema := &spec.Schema{
		VendorExtensible: spec.VendorExtensible{
			Extensions: map[string]any{
				apis.GVKExtensionKey: []map[string]any{
					{"group": "", "version": "v1", "kind": "Pod"},
				},
			},
		},
	}

	schemas := apischema.NewSchemaSetFromMap(map[string]*spec.Schema{
		"v1.Pod": podSchema,
	})

	apiResources := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "pods",
					Kind:       "Pod",
					Namespaced: true,
					Categories: []string{"all"},
				},
			},
		},
	}

	e := enricher.NewCategories(apiResources)

	err := e.Enrich(t.Context(), schemas)
	assert.NoError(t, err)

	podEntry, _ := schemas.Get("v1.Pod")
	categories := podEntry.Schema.Extensions[apis.CategoriesExtensionKey]
	assert.Equal(t, []string{"all"}, categories)
}

func TestCategoriesEnricher_NoCategories(t *testing.T) {
	podSchema := &spec.Schema{
		VendorExtensible: spec.VendorExtensible{
			Extensions: map[string]any{
				apis.GVKExtensionKey: []map[string]any{
					{"group": "", "version": "v1", "kind": "Pod"},
				},
			},
		},
	}

	schemas := apischema.NewSchemaSetFromMap(map[string]*spec.Schema{
		"v1.Pod": podSchema,
	})

	// API resource with no categories
	apiResources := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "pods",
					Kind:       "Pod",
					Namespaced: true,
					// No Categories field
				},
			},
		},
	}

	e := enricher.NewCategories(apiResources)

	err := e.Enrich(t.Context(), schemas)
	assert.NoError(t, err)

	podEntry, _ := schemas.Get("v1.Pod")
	_, hasCategories := podEntry.Schema.Extensions[apis.CategoriesExtensionKey]
	assert.False(t, hasCategories, "should not have categories extension")
}

func TestCategoriesEnricherName(t *testing.T) {
	e := enricher.NewCategories(nil)
	assert.Equal(t, "categories", e.Name())
}

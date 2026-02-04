package enricher_test

import (
	"testing"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema/enricher"
	"github.com/stretchr/testify/assert"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

func TestScopeEnricher(t *testing.T) {
	podSchema := &spec.Schema{
		VendorExtensible: spec.VendorExtensible{
			Extensions: map[string]any{
				apis.GVKExtensionKey: []map[string]any{
					{"group": "", "version": "v1", "kind": "Pod"},
				},
			},
		},
	}
	nodeSchema := &spec.Schema{
		VendorExtensible: spec.VendorExtensible{
			Extensions: map[string]any{
				apis.GVKExtensionKey: []map[string]any{
					{"group": "", "version": "v1", "kind": "Node"},
				},
			},
		},
	}

	schemas := apischema.NewSchemaSetFromMap(map[string]*spec.Schema{
		"v1.Pod":  podSchema,
		"v1.Node": nodeSchema,
	})

	// Create a REST mapper that marks Pod as namespaced, Node as cluster-scoped
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Version: "v1", Kind: "Pod"}, meta.RESTScopeNamespace)
	mapper.Add(schema.GroupVersionKind{Version: "v1", Kind: "Node"}, meta.RESTScopeRoot)

	e := enricher.NewScope(mapper)

	err := e.Enrich(t.Context(), schemas)
	assert.NoError(t, err)

	// Check Pod is namespaced
	podEntry, _ := schemas.Get("v1.Pod")
	assert.Equal(t, apiextensionsv1.NamespaceScoped, podEntry.Schema.Extensions[apis.ScopeExtensionKey])

	// Check Node is cluster-scoped
	nodeEntry, _ := schemas.Get("v1.Node")
	assert.Equal(t, apiextensionsv1.ClusterScoped, nodeEntry.Schema.Extensions[apis.ScopeExtensionKey])
}

func TestScopeEnricherName(t *testing.T) {
	e := enricher.NewScope(nil)
	assert.Equal(t, "scope", e.Name())
}

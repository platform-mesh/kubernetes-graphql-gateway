package enricher

import (
	"context"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Categories adds x-kubernetes-categories extension to schemas.
// Categories are used for grouping resources (e.g., "all" category for kubectl get all).
type Categories struct {
	resources []*metav1.APIResourceList
}

// NewCategories creates a new Categories enricher.
func NewCategories(resources []*metav1.APIResourceList) *Categories {
	return &Categories{resources: resources}
}

// Name returns the enricher name for logging.
func (e *Categories) Name() string {
	return "categories"
}

// Enrich adds category information to schemas based on API resource discovery.
func (e *Categories) Enrich(ctx context.Context, schemas *apischema.SchemaSet) error {
	logger := log.FromContext(ctx)

	for _, apiResList := range e.resources {
		gv, err := schema.ParseGroupVersion(apiResList.GroupVersion)
		if err != nil {
			logger.V(4).
				WithValues(
					"groupVersion", apiResList.GroupVersion,
					"error", err,
				).
				Info("failed to parse group version")
			continue
		}

		for _, res := range apiResList.APIResources {
			if len(res.Categories) == 0 {
				continue
			}

			entry, ok := schemas.GetByGVK(apischema.GroupVersionKind{
				Group:   gv.Group,
				Version: gv.Version,
				Kind:    res.Kind,
			})
			if !ok {
				continue
			}

			entry.Schema.AddExtension(apis.CategoriesExtensionKey, res.Categories)
		}
	}

	return nil
}

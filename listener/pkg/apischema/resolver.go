package apischema

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	ErrInvalidPath            = errors.New("path doesn't contain the / separator")
	ErrNotPreferred           = errors.New("path ApiGroup does not belong to the server preferred APIs")
	ErrGetServerPreferred     = errors.New("failed to get server preferred resources")
	ErrGetSchemaForPath       = errors.New("failed to get schema for path")
	ErrUnmarshalSchemaForPath = errors.New("failed to unmarshal schema for path")
)

type Resolver struct{}

// NewResolver creates a new Resolver
func NewResolver() *Resolver {
	return &Resolver{}
}

func (r *Resolver) Resolve(ctx context.Context, dc discovery.DiscoveryInterface, rm meta.RESTMapper) ([]byte, error) {
	logger := log.FromContext(ctx)

	apiResList, err := dc.ServerPreferredResources()
	if err != nil {
		logger.Error(err, "failed to get server preferred resources")
		return nil, errors.Join(ErrGetServerPreferred, err)
	}

	var preferredApiGroups []string
	for _, apiRes := range apiResList {
		preferredApiGroups = append(preferredApiGroups, apiRes.GroupVersion)
	}

	logger.V(4).Info("starting schema build",
		"preferredApiGroupsCount", len(preferredApiGroups),
		"apiResourceListsCount", len(apiResList))

	result, err := NewSchemaBuilder(dc.OpenAPIV3(), preferredApiGroups, logger).
		WithScope(rm).
		WithPreferredVersions(apiResList).
		WithApiResourceCategories(apiResList).
		WithRelationships().
		Complete()

	if err != nil {
		logger.Error(err, "failed to build schema",
			"preferredApiGroupsCount", len(preferredApiGroups),
			"apiResourceListsCount", len(apiResList))
		return nil, err
	}

	logger.Info("successfully resolved schema",
		"preferredApiGroupsCount", len(preferredApiGroups),
		"apiResourceListsCount", len(apiResList))

	return result, nil
}

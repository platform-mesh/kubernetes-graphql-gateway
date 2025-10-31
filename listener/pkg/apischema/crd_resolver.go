package apischema

import (
	"errors"

	"github.com/platform-mesh/golang-commons/logger"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
)

var (
	ErrInvalidPath            = errors.New("path doesn't contain the / separator")
	ErrNotPreferred           = errors.New("path ApiGroup does not belong to the server preferred APIs")
	ErrGetServerPreferred     = errors.New("failed to get server preferred resources")
	ErrGetSchemaForPath       = errors.New("failed to get schema for path")
	ErrUnmarshalSchemaForPath = errors.New("failed to unmarshal schema for path")
)

type CRDResolver struct {
	discovery.DiscoveryInterface
	meta.RESTMapper
	log *logger.Logger
}

// NewCRDResolver creates a new CRDResolver with proper logger setup
func NewCRDResolver(discovery discovery.DiscoveryInterface, restMapper meta.RESTMapper, log *logger.Logger) *CRDResolver {
	return &CRDResolver{
		DiscoveryInterface: discovery,
		RESTMapper:         restMapper,
		log:                log,
	}
}

func (cr *CRDResolver) Resolve(dc discovery.DiscoveryInterface, rm meta.RESTMapper) ([]byte, error) {
	return cr.resolveSchema(dc, rm)
}

func (cr *CRDResolver) resolveSchema(dc discovery.DiscoveryInterface, rm meta.RESTMapper) ([]byte, error) {
	apiResList, err := dc.ServerPreferredResources()
	if err != nil {
		cr.log.Error().Err(err).Msg("failed to get server preferred resources")
		return nil, errors.Join(ErrGetServerPreferred, err)
	}

	var preferredApiGroups []string
	for _, apiRes := range apiResList {
		preferredApiGroups = append(preferredApiGroups, apiRes.GroupVersion)
	}

	result, err := NewSchemaBuilder(dc.OpenAPIV3(), preferredApiGroups, cr.log).
		WithScope(rm).
		WithPreferredVersions(apiResList).
		WithApiResourceCategories(apiResList).
		WithRelationships().
		Complete()

	if err != nil {
		cr.log.Error().Err(err).
			Int("preferredApiGroupsCount", len(preferredApiGroups)).
			Int("apiResourceListsCount", len(apiResList)).
			Msg("failed to build schema")
		return nil, err
	}

	cr.log.Debug().
		Int("preferredApiGroupsCount", len(preferredApiGroups)).
		Int("apiResourceListsCount", len(apiResList)).
		Msg("successfully resolved schema")

	return result, nil
}

package apischema

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/platform-mesh/golang-commons/logger"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/validation/spec"
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

func getSchemaForPath(preferredApiGroups []string, path string, gv openapi.GroupVersion) (map[string]*spec.Schema, error) {
	if !strings.Contains(path, separator) {
		return nil, ErrInvalidPath
	}
	pathApiGroupArray := strings.Split(path, separator)
	pathApiGroup := strings.Join(pathApiGroupArray[1:], separator)
	// filer out apiGroups that aren't in the preferred list
	if !slices.Contains(preferredApiGroups, pathApiGroup) {
		return nil, ErrNotPreferred
	}

	b, err := gv.Schema(discovery.AcceptV1)
	if err != nil {
		return nil, errors.Join(ErrGetSchemaForPath, err)
	}

	resp := &schemaResponse{}
	if err := json.Unmarshal(b, resp); err != nil {
		return nil, errors.Join(ErrUnmarshalSchemaForPath, err)
	}
	return resp.Components.Schemas, nil
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

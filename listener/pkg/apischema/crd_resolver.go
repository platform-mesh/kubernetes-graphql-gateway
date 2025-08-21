package apischema

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/openmfp/golang-commons/logger"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

var (
	ErrInvalidPath              = errors.New("path doesn't contain the / separator")
	ErrNotPreferred             = errors.New("path ApiGroup does not belong to the server preferred APIs")
	ErrGVKNotPreferred          = errors.New("failed to find CRD GVK in API preferred resources")
	ErrGetServerPreferred       = errors.New("failed to get server preferred resources")
	ErrFilterPreferredResources = errors.New("failed to filter server preferred resources")
	ErrGetSchemaForPath         = errors.New("failed to get schema for path")
	ErrUnmarshalSchemaForPath   = errors.New("failed to unmarshal schema for path")
)

type GroupKindVersions struct {
	*metav1.GroupKind
	Versions []string
}

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

func (cr *CRDResolver) ResolveApiSchema(crd *apiextensionsv1.CustomResourceDefinition) ([]byte, error) {
	gkv := getCRDGroupKindVersions(crd.Spec)

	apiResLists, err := cr.ServerPreferredResources()
	if err != nil {
		cr.log.Error().Err(err).
			Str("crdName", crd.Name).
			Str("group", gkv.Group).
			Str("kind", gkv.Kind).
			Msg("failed to get server preferred resources")
		return nil, errors.Join(ErrGetServerPreferred, err)
	}

	preferredApiGroups, err := cr.errorIfCRDNotInPreferredApiGroups(gkv, apiResLists)
	if err != nil {
		cr.log.Error().Err(err).
			Str("crdName", crd.Name).
			Str("group", gkv.Group).
			Str("kind", gkv.Kind).
			Msg("failed to filter preferred resources")
		return nil, errors.Join(ErrFilterPreferredResources, err)
	}

	result, err := NewSchemaBuilder(cr.OpenAPIV3(), preferredApiGroups, cr.log).
		WithScope(cr.RESTMapper).
		WithPreferredVersions(apiResLists).
		WithCRDCategories(crd).
		WithRelationships().
		Complete()

	if err != nil {
		cr.log.Error().Err(err).
			Str("crdName", crd.Name).
			Int("preferredApiGroupsCount", len(preferredApiGroups)).
			Msg("failed to complete schema building")
		return nil, err
	}

	cr.log.Debug().
		Str("crdName", crd.Name).
		Str("group", gkv.Group).
		Str("kind", gkv.Kind).
		Msg("successfully resolved API schema")

	return result, nil
}

func (cr *CRDResolver) errorIfCRDNotInPreferredApiGroups(gkv *GroupKindVersions, apiResLists []*metav1.APIResourceList) ([]string, error) {
	isKindFound := false
	preferredApiGroups := make([]string, 0, len(apiResLists))

	for _, apiResources := range apiResLists {
		gv, err := schema.ParseGroupVersion(apiResources.GroupVersion)
		if err != nil {
			cr.log.Error().Err(err).
				Str("groupVersion", apiResources.GroupVersion).
				Str("targetGroup", gkv.Group).
				Str("targetKind", gkv.Kind).
				Msg("failed to parse group version")
			continue
		}

		isGroupFound := gkv.Group == gv.Group
		isVersionFound := slices.Contains(gkv.Versions, gv.Version)

		if isGroupFound && isVersionFound && !isKindFound {
			isKindFound = isCRDKindIncluded(gkv, apiResources)
			cr.log.Debug().
				Str("groupVersion", apiResources.GroupVersion).
				Str("targetGroup", gkv.Group).
				Str("targetKind", gkv.Kind).
				Bool("kindFound", isKindFound).
				Msg("checking if CRD kind is included in preferred APIs")
		}

		preferredApiGroups = append(preferredApiGroups, apiResources.GroupVersion)
	}

	if !isKindFound {
		cr.log.Warn().
			Str("group", gkv.Group).
			Str("kind", gkv.Kind).
			Strs("versions", gkv.Versions).
			Int("checkedApiGroups", len(preferredApiGroups)).
			Msg("CRD kind not found in preferred API resources")
		return nil, ErrGVKNotPreferred
	}

	cr.log.Debug().
		Str("group", gkv.Group).
		Str("kind", gkv.Kind).
		Int("preferredApiGroupsCount", len(preferredApiGroups)).
		Msg("successfully found CRD in preferred API groups")

	return preferredApiGroups, nil
}

func isCRDKindIncluded(gvk *GroupKindVersions, apiResources *metav1.APIResourceList) bool {
	for _, res := range apiResources.APIResources {
		if res.Kind == gvk.Kind {
			return true
		}
	}
	return false
}

func getCRDGroupKindVersions(spec apiextensionsv1.CustomResourceDefinitionSpec) *GroupKindVersions {
	versions := make([]string, 0, len(spec.Versions))
	for _, v := range spec.Versions {
		versions = append(versions, v.Name)
	}
	return &GroupKindVersions{
		GroupKind: &metav1.GroupKind{
			Group: spec.Group,
			Kind:  spec.Names.Kind,
		},
		Versions: versions,
	}
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

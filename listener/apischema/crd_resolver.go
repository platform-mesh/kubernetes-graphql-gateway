package apischema

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

var (
	ErrInvalidPath     = errors.New("path doesn't contain the / separator")
	ErrNotPreferred    = errors.New("path ApiGroup does not belong to the server preferred APIs")
	ErrGVKNotPreferred = errors.New("failed to find CRD GVK in API preferred resources")
)

type GroupKindVersions struct {
	*metav1.GroupKind
	Versions []string
}

type CRDResolver struct {
	*discovery.DiscoveryClient
	meta.RESTMapper
}

func (cr *CRDResolver) Resolve() ([]byte, error) {
	return resolveSchema(cr.DiscoveryClient, cr.RESTMapper)
}

func (cr *CRDResolver) ResolveApiSchema(crd *apiextensionsv1.CustomResourceDefinition) ([]byte, error) {
	gkv := getCRDGroupKindVersions(crd.Spec)

	apiResLists, err := cr.ServerPreferredResources()
	if err != nil {
		return nil, fmt.Errorf("failed to get server preferred resources: %w", err)
	}

	preferredApiGroups, err := errorIfCRDNotInPreferredApiGroups(gkv, apiResLists)
	if err != nil {
		return nil, fmt.Errorf("failed to filter server preferred resources: %w", err)
	}

	return resolveForPaths(cr.OpenAPIV3(), preferredApiGroups, cr.RESTMapper)
}

func errorIfCRDNotInPreferredApiGroups(gkv *GroupKindVersions, apiResLists []*metav1.APIResourceList) ([]string, error) {
	isKindFound := false
	preferredApiGroups := make([]string, 0, len(apiResLists))
	for _, apiResources := range apiResLists {
		gv, err := schema.ParseGroupVersion(apiResources.GroupVersion)
		if err != nil {
			//TODO: debug log?
			continue
		}
		isGroupFound := gkv.Group == gv.Group
		isVersionFound := slices.Contains(gkv.Versions, gv.Version)
		if isGroupFound && isVersionFound && !isKindFound {
			isKindFound = isCRDKindIncluded(gkv, apiResources)
		}
		preferredApiGroups = append(preferredApiGroups, apiResources.GroupVersion)
	}
	if !isKindFound {
		return nil, ErrGVKNotPreferred
	}
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
		return nil, fmt.Errorf("failed to get schema for path %s :%w", path, err)
	}

	resp := &schemaResponse{}
	if err := json.Unmarshal(b, resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema for path %s :%w", path, err)
	}
	return resp.Components.Schemas, nil
}

func resolveForPaths(oc openapi.Client, preferredApiGroups []string, rm meta.RESTMapper) ([]byte, error) {
	apiv3Paths, err := oc.Paths()
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenAPI paths: %w", err)
	}

	schemas := make(map[string]*spec.Schema)
	for path, gv := range apiv3Paths {
		schema, err := getSchemaForPath(preferredApiGroups, path, gv)
		if err != nil {
			//TODO: debug log?
			continue
		}
		maps.Copy(schemas, schema)
	}

	scopedSchemas, err := addScopeInfo(schemas, rm)
	if err != nil {
		return nil, fmt.Errorf("failed to add scope info to v3 schema: %w", err)
	}

	v3JSON, err := json.Marshal(&schemaResponse{
		Components: schemasComponentsWrapper{
			Schemas: scopedSchemas,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal openAPI v3 schema: %w", err)
	}
	v2JSON, err := ConvertJSON(v3JSON)
	if err != nil {
		return nil, fmt.Errorf("failed to convert openAPI v3 schema to v2: %w", err)
	}

	return v2JSON, nil
}

func resolveSchema(dc discovery.DiscoveryInterface, rm meta.RESTMapper) ([]byte, error) {
	preferredApiGroups := []string{}
	apiResList, err := dc.ServerPreferredResources()
	if err != nil {
		return nil, fmt.Errorf("failed to get server preferred resources: %w", err)
	}
	for _, apiRes := range apiResList {
		preferredApiGroups = append(preferredApiGroups, apiRes.GroupVersion)
	}

	return resolveForPaths(dc.OpenAPIV3(), preferredApiGroups, rm)
}

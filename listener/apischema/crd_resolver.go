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
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

var (
	ErrInvalidPath     = errors.New("path doesn't contain the / separator")
	ErrNotPreferred    = errors.New("path ApiGroup does not belong to the server preferred APIs")
	ErrGVKNotPreferred = errors.New("failed to find CRD GVK in API preferred resources")
)

type CRDResolver struct {
	*discovery.DiscoveryClient
}

func (cr *CRDResolver) Resolve() ([]byte, error) {
	return resolveSchema(cr.DiscoveryClient)
}

func (cr *CRDResolver) ResolveApiSchema(crd *apiextensionsv1.CustomResourceDefinition) ([]byte, error) {
	gvk, err := getCRDGroupVersionKind(crd.Spec)
	if err != nil {
		return nil, fmt.Errorf("failed to get CRD GVK: %w", err)
	}

	apiResLists, err := cr.ServerPreferredResources()
	if err != nil {
		return nil, fmt.Errorf("failed to get server preferred resources: %w", err)
	}

	preferredApiGroups, err := errorIfCRDNotInPreferredApiGroups(gvk, apiResLists)
	if err != nil {
		return nil, fmt.Errorf("failed to filter server preferred resources: %w", err)
	}

	return resolveForPaths(cr.OpenAPIV3(), preferredApiGroups)
}

func errorIfCRDNotInPreferredApiGroups(gvk *metav1.GroupVersionKind, apiResLists []*metav1.APIResourceList) ([]string, error) {
	targetGV := gvk.Group + "/" + gvk.Version
	isGVFound := false
	preferredApiGroups := make([]string, 0, len(apiResLists))
	for _, apiResources := range apiResLists {
		gv := apiResources.GroupVersion
		isGVFound = gv == targetGV
		if isGVFound && !isCRDKindIncluded(gvk, apiResources) {
			return nil, ErrGVKNotPreferred
		}
		preferredApiGroups = append(preferredApiGroups, gv)
	}

	if !isGVFound {
		return nil, ErrGVKNotPreferred
	}
	return preferredApiGroups, nil
}

func isCRDKindIncluded(gvk *metav1.GroupVersionKind, apiResources *metav1.APIResourceList) bool {
	for _, res := range apiResources.APIResources {
		if res.Kind == gvk.Kind {
			return true
		}
	}
	return false
}

func getCRDGroupVersionKind(spec apiextensionsv1.CustomResourceDefinitionSpec) (*metav1.GroupVersionKind, error) {
	for _, v := range spec.Versions {
		if v.Storage {
			return &metav1.GroupVersionKind{
				Group:   spec.Group,
				Version: v.Name,
				Kind:    spec.Names.Kind,
			}, nil
		}
	}
	return nil, errors.New("failed to find storage version")
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

func resolveForPaths(oc openapi.Client, preferredApiGroups []string) ([]byte, error) {
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
	v3JSON, err := json.Marshal(&schemaResponse{
		Components: schemasComponentsWrapper{
			Schemas: schemas,
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

func resolveSchema(dc discovery.DiscoveryInterface) ([]byte, error) {
	preferredApiGroups := []string{}
	apiResList, err := dc.ServerPreferredResources()
	if err != nil {
		return nil, fmt.Errorf("failed to get server preferred resources: %w", err)
	}
	for _, apiRes := range apiResList {
		preferredApiGroups = append(preferredApiGroups, apiRes.GroupVersion)
	}

	return resolveForPaths(dc.OpenAPIV3(), preferredApiGroups)
}

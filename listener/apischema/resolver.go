package apischema

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	//nolint:staticcheck // SA1019 Keep using module since it's still being maintained and the api of google.golang.org/protobuf/proto differs

	"k8s.io/client-go/discovery"
	"k8s.io/kube-openapi/pkg/validation/spec"
	// "github.com/getkin/kin-openapi/openapi2conv"
	// "github.com/getkin/kin-openapi/openapi3"
)

const (
	separator = "/"
)

type schemasComponentsWrapper struct {
	Schemas map[string]*spec.Schema `json:"schemas"`
}

type schemaResponse struct {
	Components schemasComponentsWrapper `json:"components"`
}

type Resolver interface {
	Resolve(dc discovery.DiscoveryInterface) ([]byte, error)
}

func NewResolver() *ResolverImpl {
	return &ResolverImpl{}
}

type ResolverImpl struct {
}

func (r *ResolverImpl) Resolve(dc discovery.DiscoveryInterface) ([]byte, error) {
	preferredApiGroups := []string{}
	apiResList, err := dc.ServerPreferredResources()
	if err != nil {
		return nil, fmt.Errorf("failed to get server preferred resources: %w", err)
	}
	for _, apiRes := range apiResList {
		preferredApiGroups = append(preferredApiGroups, apiRes.GroupVersion)
	}

	apiv3Paths, err := dc.OpenAPIV3().Paths()
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenAPI paths: %w", err)
	}

	schemas := make(map[string]*spec.Schema)
	for key, path := range apiv3Paths {
		if !strings.Contains(key, separator) {
			continue
		}
		pathApiGroupArray := strings.Split(key, separator)
		pathApiGroup := strings.Join(pathApiGroupArray[1:], separator)
		// filer out apiGroups that aren't in the preferred list
		if !slices.Contains(preferredApiGroups, pathApiGroup) {
			continue
		}

		b, err := path.Schema(discovery.AcceptV1)
		if err != nil {
			//TODO: debug log?
			continue
		}

		resp := &schemaResponse{}
		err = json.Unmarshal(b, resp)
		if err != nil {
			//TODO: debug log?
			continue
		}
		maps.Copy(schemas, resp.Components.Schemas)
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

package apischema

import (
	"errors"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/kube-openapi/pkg/validation/spec"
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

func NewResolver(rm meta.RESTMapper) (*ResolverImpl, error) {
	if rm == nil {
		return nil, errors.New("rest mapper might not be nil")
	}
	return &ResolverImpl{RESTMapper: rm}, nil
}

type ResolverImpl struct {
	meta.RESTMapper
}

func (r *ResolverImpl) Resolve(dc discovery.DiscoveryInterface) ([]byte, error) {
	return resolveSchema(dc, r.RESTMapper)
}

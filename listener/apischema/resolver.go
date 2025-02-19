package apischema

import (
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

func NewResolver() *ResolverImpl {
	return &ResolverImpl{}
}

type ResolverImpl struct {
}

func (r *ResolverImpl) Resolve(dc discovery.DiscoveryInterface) ([]byte, error) {
	return resolveSchema(dc)
}

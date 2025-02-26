package apischema

import (
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
	Resolve(dc discovery.DiscoveryInterface, rm meta.RESTMapper) ([]byte, error)
}

type resolverImpl struct {
}

func NewResolver() *resolverImpl {
	return &resolverImpl{}
}

func (r *resolverImpl) Resolve(dc discovery.DiscoveryInterface, rm meta.RESTMapper) ([]byte, error) {
	return resolveSchema(dc, rm)
}

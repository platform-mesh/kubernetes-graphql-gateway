package apischema

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/kube-openapi/pkg/validation/spec"

	"github.com/openmfp/golang-commons/logger"
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

type ResolverProvider struct {
	log *logger.Logger
}

func NewResolver(log *logger.Logger) *ResolverProvider {
	return &ResolverProvider{log: log}
}

func (r *ResolverProvider) Resolve(dc discovery.DiscoveryInterface, rm meta.RESTMapper) ([]byte, error) {
	crdResolver := NewCRDResolver(dc, rm, r.log)
	return crdResolver.resolveSchema(dc, rm)
}

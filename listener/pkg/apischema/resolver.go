package apischema

import (
	"github.com/platform-mesh/golang-commons/logger"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
)

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

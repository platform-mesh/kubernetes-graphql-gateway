package roundtripper

import (
	"net/http"
	"strings"

	utilscontext "github.com/platform-mesh/kubernetes-graphql-gateway/gateway/utils/context"
)

type PathTemplateHandler struct {
	inner    http.RoundTripper
	prefix   string
	basePath string
}

func NewPathTemplateHandler(inner http.RoundTripper, prefix, basePath string) *PathTemplateHandler {
	return &PathTemplateHandler{inner: inner, prefix: prefix, basePath: basePath}
}

func (h *PathTemplateHandler) RoundTrip(req *http.Request) (*http.Response, error) {
	if h.prefix == "" {
		return h.inner.RoundTrip(req)
	}

	target, ok := utilscontext.GetClusterTargetFromCtx(req.Context())
	if !ok || target == "" {
		return h.inner.RoundTrip(req)
	}

	resolved := strings.ReplaceAll(h.prefix, "{clusterTarget}", target)
	newReq := req.Clone(req.Context())
	newReq.URL.Path = resolved + strings.TrimPrefix(req.URL.Path, h.basePath)

	return h.inner.RoundTrip(newReq)
}

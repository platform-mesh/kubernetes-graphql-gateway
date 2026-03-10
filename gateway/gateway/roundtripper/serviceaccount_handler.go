package roundtripper

import (
	"net/http"

	utilscontext "github.com/platform-mesh/kubernetes-graphql-gateway/gateway/utils/context"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ServiceAccountHandler generates service account tokens for requests.
// If a user-provided token exists in the context, it passes to the next handler.
// Otherwise, it generates a token using the ServiceAccount TokenRequest API.
// This is a terminal handler when SA auth is configured.
type ServiceAccountHandler struct {
	baseRT    http.RoundTripper
	k8sClient client.Client
	saConfig  ServiceAccountConfig
	saRT      *serviceAccountRoundTripper
}

// NewServiceAccountHandler creates a handler that generates SA tokens.
func NewServiceAccountHandler(baseRT http.RoundTripper, k8sClient client.Client, saConfig ServiceAccountConfig) *ServiceAccountHandler {
	return &ServiceAccountHandler{
		baseRT:    baseRT,
		k8sClient: k8sClient,
		saConfig:  saConfig,
		saRT: &serviceAccountRoundTripper{
			delegate:  baseRT,
			k8sClient: k8sClient,
			saConfig:  saConfig,
		},
	}
}

// RoundTrip implements union.Handler.
func (h *ServiceAccountHandler) RoundTrip(req *http.Request) (*http.Response, error, bool) {
	ctx := req.Context()
	logger := log.FromContext(ctx)

	// If user already provided a token, use that instead of SA token
	if token, ok := utilscontext.GetTokenFromCtx(ctx); ok && token != "" {
		logger.V(4).WithValues("path", req.URL.Path).Info("User token found, using bearer auth instead of SA")
		req = utilnet.CloneRequest(req)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := h.baseRT.RoundTrip(req)
		return resp, err, true
	}

	// No user token, use ServiceAccount token
	logger.V(4).WithValues("path", req.URL.Path).Info("Using ServiceAccount authentication")
	resp, err := h.saRT.RoundTrip(req)
	return resp, err, true
}

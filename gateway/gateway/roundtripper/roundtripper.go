package roundtripper

import (
	"net/http"
	"strings"

	utilscontext "github.com/platform-mesh/kubernetes-graphql-gateway/gateway/utils/context"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type roundTripper struct {
	adminRT, baseRT, unauthorizedRT http.RoundTripper
	developmentDisableAuth          bool
}

type unauthorizedRoundTripper struct{}

func New(adminRoundTripper, baseRoundTripper, unauthorizedRT http.RoundTripper, developmentDisableAuth bool) http.RoundTripper {
	return &roundTripper{
		adminRT:                adminRoundTripper,
		unauthorizedRT:         unauthorizedRT,
		baseRT:                 baseRoundTripper,
		developmentDisableAuth: developmentDisableAuth,
	}
}

// NewUnauthorizedRoundTripper returns a RoundTripper that always returns 401 Unauthorized
func NewUnauthorizedRoundTripper() http.RoundTripper {
	return &unauthorizedRoundTripper{}
}

// NewBaseRoundTripper creates a base HTTP transport with only TLS configuration (no authentication)Add a comment on  line R42Add diff commentMarkdown input:  edit mode selected.WritePreviewAdd a suggestionHeadingBoldItalicQuoteCodeLinkUnordered listNumbered listTask listMentionReferenceSaved repliesAdd FilesPaste, drop, or click to add filesCancelCommentStart a reviewReturn to code
func NewBaseRoundTripper(tlsConfig rest.TLSClientConfig) (http.RoundTripper, error) {
	return rest.TransportFor(&rest.Config{
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   tlsConfig.Insecure,
			ServerName: tlsConfig.ServerName,
			CAData:     tlsConfig.CAData,
		},
	})
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	logger := log.FromContext(ctx)
	logger.V(4).WithValues(
		"req.Host", req.Host,
		"req.URL.Host", req.URL.Host,
		"path", req.URL.Path,
		"method", req.Method,
		"disableAuth", rt.developmentDisableAuth).
		Info("RoundTripper processing request")

	if rt.developmentDisableAuth {
		logger.V(4).WithValues("path", req.URL.Path).Info("Local development mode, using admin credentials")
		return rt.adminRT.RoundTrip(req)
	}

	// client-go sends discovery requests to the Kubernetes API server before any CRUD request.
	// And it doesn't attach any authentication token to these requests, even if we put token into the context at ServeHTTP method.
	// That is why we don't protect discovery requests with authentication.
	if isDiscoveryRequest(req) {
		logger.V(4).WithValues("path", req.URL.Path).Info("Discovery request detected, allowing with admin credentials")
		return rt.adminRT.RoundTrip(req)
	}

	token, ok := utilscontext.GetTokenFromCtx(ctx)
	if !ok || token == "" {
		logger.V(4).WithValues("path", req.URL.Path).Error(nil, "No token found for resource request, denying")
		return rt.unauthorizedRT.RoundTrip(req)
	}

	// No we are going to use token based auth only, so we are reassigning the headers
	req = utilnet.CloneRequest(req)
	req.Header.Del("Authorization")

	logger.V(4).WithValues("path", req.URL.Path).Info("Using bearer token authentication")
	return transport.NewBearerAuthRoundTripper(token, rt.baseRT).RoundTrip(req)

}

func (u *unauthorizedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusUnauthorized,
		Request:    req,
		Body:       http.NoBody,
	}, nil
}

func isDiscoveryRequest(req *http.Request) bool {
	// Only GET requests can be discovery requests
	if req.Method != http.MethodGet {
		return false
	}

	// Parse and clean the URL path
	path := req.URL.Path
	path = strings.Trim(path, "/") // remove leading and trailing slashes
	if path == "" {
		return false
	}
	parts := strings.Split(path, "/")

	// Remove workspace prefixes to get the actual API path
	if len(parts) >= 5 && parts[0] == "services" && parts[2] == "clusters" {
		// Handle virtual workspace prefixes first: /services/<service>/clusters/<workspace>/api
		parts = parts[4:] // Remove /services/<service>/clusters/<workspace> prefix
	} else if len(parts) >= 3 && parts[0] == "clusters" {
		// Handle KCP workspace prefixes: /clusters/<workspace>/api
		parts = parts[2:] // Remove /clusters/<workspace> prefix
	}

	// Check if the remaining path matches Kubernetes discovery API patterns
	switch {
	case len(parts) == 1 && (parts[0] == "api" || parts[0] == "apis"):
		return true // /api or /apis (root discovery endpoints)
	case len(parts) == 2 && parts[0] == "apis":
		return true // /apis/<group> (group discovery)
	case len(parts) == 2 && parts[0] == "api":
		return true // /api/v1 (core API version discovery)
	case len(parts) == 3 && parts[0] == "apis":
		return true // /apis/<group>/<version> (group version discovery)
	default:
		return false
	}
}

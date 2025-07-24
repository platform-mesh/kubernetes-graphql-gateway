package roundtripper

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/openmfp/golang-commons/logger"
	"k8s.io/client-go/transport"

	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
)

type TokenKey struct{}

type roundTripper struct {
	log                     *logger.Logger
	adminRT, unauthorizedRT http.RoundTripper
	appCfg                  config.Config
}

type unauthorizedRoundTripper struct{}

func New(log *logger.Logger, appCfg config.Config, adminRoundTripper, unauthorizedRT http.RoundTripper) http.RoundTripper {
	return &roundTripper{
		log:            log,
		adminRT:        adminRoundTripper,
		unauthorizedRT: unauthorizedRT,
		appCfg:         appCfg,
	}
}

// NewUnauthorizedRoundTripper returns a RoundTripper that always returns 401 Unauthorized
func NewUnauthorizedRoundTripper() http.RoundTripper {
	return &unauthorizedRoundTripper{}
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.log.Info().
		Str("req.Host", req.Host).
		Str("req.URL.Host", req.URL.Host).
		Str("path", req.URL.Path).
		Str("method", req.Method).
		Bool("shouldImpersonate", rt.appCfg.Gateway.ShouldImpersonate).
		Str("usernameClaim", rt.appCfg.Gateway.UsernameClaim).
		Msg("RoundTripper processing request")

	if rt.appCfg.LocalDevelopment {
		rt.log.Debug().Str("path", req.URL.Path).Msg("Local development mode, using admin credentials")
		return rt.adminRT.RoundTrip(req)
	}

	// client-go sends discovery requests to the Kubernetes API server before any CRUD request.
	// And it doesn't attach any authentication token to these requests, even if we put token into the context at ServeHTTP method.
	// That is why we don't protect discovery requests with authentication.
	if isDiscoveryRequest(req) {
		rt.log.Debug().Str("path", req.URL.Path).Msg("Discovery request detected, allowing with admin credentials")
		return rt.adminRT.RoundTrip(req)
	}

	token, ok := req.Context().Value(TokenKey{}).(string)
	if !ok || token == "" {
		rt.log.Error().Str("path", req.URL.Path).Msg("No token found for resource request, denying")
		return rt.unauthorizedRT.RoundTrip(req)
	}

	// No we are going to use token based auth only, so we are reassigning the headers
	req.Header.Del("Authorization")
	req.Header.Set("Authorization", "Bearer "+token)

	if !rt.appCfg.Gateway.ShouldImpersonate {
		rt.log.Debug().Str("path", req.URL.Path).Msg("Using bearer token authentication")

		return rt.adminRT.RoundTrip(req)
	}

	// Impersonation mode: extract user from token and impersonate
	rt.log.Debug().Str("path", req.URL.Path).Msg("Using impersonation mode")
	claims := jwt.MapClaims{}
	_, _, err := jwt.NewParser().ParseUnverified(token, claims)
	if err != nil {
		rt.log.Error().Err(err).Str("path", req.URL.Path).Msg("Failed to parse token for impersonation, denying request")
		return rt.unauthorizedRT.RoundTrip(req)
	}

	userNameRaw, ok := claims[rt.appCfg.Gateway.UsernameClaim]
	if !ok {
		rt.log.Error().Str("path", req.URL.Path).Str("usernameClaim", rt.appCfg.Gateway.UsernameClaim).Msg("No user claim found in token for impersonation, denying request")
		return rt.unauthorizedRT.RoundTrip(req)
	}

	userName, ok := userNameRaw.(string)
	if !ok || userName == "" {
		rt.log.Error().Str("path", req.URL.Path).Str("usernameClaim", rt.appCfg.Gateway.UsernameClaim).Msg("User claim is not a valid string for impersonation, denying request")
		return rt.unauthorizedRT.RoundTrip(req)
	}

	rt.log.Debug().Str("path", req.URL.Path).Str("impersonateUser", userName).Msg("Impersonating user")

	impersonatingRT := transport.NewImpersonatingRoundTripper(transport.ImpersonationConfig{
		UserName: userName,
	}, rt.adminRT)

	return impersonatingRT.RoundTrip(req)
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

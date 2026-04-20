package roundtripper

import (
	"net/http"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DiscoveryHandler handles Kubernetes discovery requests using admin credentials.
type DiscoveryHandler struct {
	adminRT http.RoundTripper
}

// NewDiscoveryHandler creates a handler for Kubernetes discovery requests.
func NewDiscoveryHandler(adminRT http.RoundTripper) *DiscoveryHandler {
	return &DiscoveryHandler{adminRT: adminRT}
}

// RoundTrip implements union.Handler.
func (h *DiscoveryHandler) RoundTrip(req *http.Request) (*http.Response, error, bool) {
	if !isDiscoveryRequest(req) {
		return nil, nil, false
	}

	logger := log.FromContext(req.Context())
	logger.V(4).WithValues("path", req.URL.Path).Info("Discovery request detected, allowing with admin credentials")

	resp, err := h.adminRT.RoundTrip(req)
	return resp, err, true
}

func isDiscoveryRequest(req *http.Request) bool {
	if req.Method != http.MethodGet {
		return false
	}

	path := req.URL.Path
	path = strings.Trim(path, "/")
	if path == "" {
		return false
	}
	parts := strings.Split(path, "/")

	// Strip any path prefix before the Kubernetes API segments.
	// Scan from the end so that a prefix segment literally named "api" or "apis"
	// (e.g. /services/api/clusters/cl/api/v1) doesn't shadow the real K8s root.
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == "api" || parts[i] == "apis" || parts[i] == "openapi" {
			parts = parts[i:]
			break
		}
	}

	switch {
	case len(parts) == 1 && (parts[0] == "api" || parts[0] == "apis"):
		return true
	case len(parts) == 2 && parts[0] == "apis":
		return true
	case len(parts) == 2 && parts[0] == "api":
		return true
	case len(parts) == 3 && parts[0] == "apis":
		return true
	case len(parts) >= 1 && parts[0] == "openapi":
		return true
	default:
		return false
	}
}

package roundtripper

import "net/http"

// NewUnauthorizedRoundTripperForTest creates an unauthorizedRoundTripper for testing
func NewUnauthorizedRoundTripperForTest() http.RoundTripper {
	return &unauthorizedRoundTripper{}
}

// IsWorkspaceQualifiedForTest exports isWorkspaceQualified for testing
func IsWorkspaceQualifiedForTest(path string) bool {
	return isWorkspaceQualified(path)
}

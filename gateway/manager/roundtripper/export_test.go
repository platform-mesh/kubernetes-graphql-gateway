package roundtripper

import "net/http"

// NewUnauthorizedRoundTripperForTest creates an unauthorizedRoundTripper for testing
func NewUnauthorizedRoundTripperForTest() http.RoundTripper {
	return &unauthorizedRoundTripper{}
}

package roundtripper

import (
	"net/http"

	"k8s.io/client-go/rest"
)

// unauthorizedRoundTripper always returns 401 Unauthorized responses.
type unauthorizedRoundTripper struct{}

// NewUnauthorizedRoundTripper returns a RoundTripper that always returns 401 Unauthorized.
func NewUnauthorizedRoundTripper() http.RoundTripper {
	return &unauthorizedRoundTripper{}
}

func (u *unauthorizedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusUnauthorized,
		Request:    req,
		Body:       http.NoBody,
	}, nil
}

// NewBaseRoundTripper creates a base HTTP transport with only TLS configuration (no authentication).
func NewBaseRoundTripper(tlsConfig rest.TLSClientConfig) (http.RoundTripper, error) {
	return rest.TransportFor(&rest.Config{
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   tlsConfig.Insecure,
			ServerName: tlsConfig.ServerName,
			CAData:     tlsConfig.CAData,
		},
	})
}

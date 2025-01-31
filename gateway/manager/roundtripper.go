package manager

import (
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/openmfp/golang-commons/logger"
	"k8s.io/client-go/transport"
)

type TokenKey struct{}

type roundTripper struct {
	log  *logger.Logger
	base http.RoundTripper // TODO change to awareBaseHttp
}

func NewRoundTripper(log *logger.Logger, base http.RoundTripper) http.RoundTripper {
	return &roundTripper{
		log:  log,
		base: base,
	}
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	token, ok := req.Context().Value(TokenKey{}).(string)
	if !ok {
		rt.log.Debug().Msg("No token found in context")
		return rt.base.RoundTrip(req)
	}

	claims := jwt.MapClaims{}
	_, _, err := jwt.NewParser().ParseUnverified(token, claims)
	if err != nil {
		rt.log.Error().Err(err).Msg("Failed to parse token")
		return rt.base.RoundTrip(req)
	}

	t := transport.NewImpersonatingRoundTripper(transport.ImpersonationConfig{
		UserName: claims["email"].(string),
	}, rt.base)

	return t.RoundTrip(req)
}

package manager

import (
	"github.com/openmfp/golang-commons/logger"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"net/http"
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
		rt.log.Debug().Str("requestURI", req.RequestURI).Msg("No token found in context")
		return rt.base.RoundTrip(req)
	}

	rt.log.Debug().Str("requestURI", req.RequestURI).Msg("Adding token to request")

	req = utilnet.CloneRequest(req)
	req.Header.Set("Authorization", "Bearer "+token)

	return rt.base.RoundTrip(req)
}

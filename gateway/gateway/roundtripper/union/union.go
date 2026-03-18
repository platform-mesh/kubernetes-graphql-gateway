package union

import (
	"errors"
	"net/http"
)

// Handler processes HTTP requests and indicates whether it handled the request.
type Handler interface {
	RoundTrip(req *http.Request) (resp *http.Response, err error, handled bool)
}

type roundTripperUnion struct {
	handlers []Handler
}

func (u *roundTripperUnion) RoundTrip(req *http.Request) (*http.Response, error) {
	for _, h := range u.handlers {
		resp, err, handled := h.RoundTrip(req)
		if handled {
			return resp, err
		}
	}
	return nil, errors.New("no handler processed the request")
}

var _ http.RoundTripper = &roundTripperUnion{}

// New creates a union roundtripper from the given handlers.
// Handlers are tried in order until one handles the request.
func New(handlers ...Handler) http.RoundTripper {
	if len(handlers) == 1 {
		return &singleHandler{h: handlers[0]}
	}
	return &roundTripperUnion{handlers: handlers}
}

type singleHandler struct {
	h Handler
}

func (s *singleHandler) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err, _ := s.h.RoundTrip(req)
	return resp, err
}

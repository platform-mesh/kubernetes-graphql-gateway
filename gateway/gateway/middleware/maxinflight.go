package middleware

import "net/http"

// WithMaxInFlightRequests returns a middleware that limits the number of concurrent
// requests being processed. When the limit is reached, new requests receive a
// 429 Too Many Requests response. Setting maxInFlight to 0 or less disables the limit.
func WithMaxInFlightRequests(handler http.Handler, maxInFlight int) http.Handler {
	if maxInFlight <= 0 {
		return handler
	}
	sem := make(chan struct{}, maxInFlight)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			handler.ServeHTTP(w, r)
		default:
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		}
	})
}

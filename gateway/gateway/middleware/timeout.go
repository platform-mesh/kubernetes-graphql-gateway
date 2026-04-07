package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"sync"
	"time"
)

// WithTimeout returns a middleware that enforces a request timeout.
// When the timeout is exceeded, it responds with 504 Gateway Timeout and a
// GraphQL-formatted JSON error body. Setting timeout to 0 or less disables the limit.
func WithTimeout(handler http.Handler, timeout time.Duration) http.Handler {
	if timeout <= 0 {
		return handler
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		tw := &timeoutWriter{header: make(http.Header)}
		done := make(chan struct{})
		go func() {
			handler.ServeHTTP(tw, r.WithContext(ctx))
			close(done)
		}()

		select {
		case <-done:
			tw.mu.Lock()
			defer tw.mu.Unlock()
			dst := w.Header()
			maps.Copy(dst, tw.header)
			if tw.code != 0 {
				w.WriteHeader(tw.code)
			}
			w.Write(tw.buf.Bytes()) //nolint:errcheck
		case <-ctx.Done():
			tw.mu.Lock()
			tw.timedOut = true
			tw.mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusGatewayTimeout)
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"errors": []map[string]string{
					{"message": "request timeout"},
				},
			})
		}
	})
}

// timeoutWriter buffers the response so it can be discarded on timeout.
type timeoutWriter struct {
	header   http.Header
	buf      bytes.Buffer
	code     int
	mu       sync.Mutex
	timedOut bool
}

func (tw *timeoutWriter) Header() http.Header {
	return tw.header
}

func (tw *timeoutWriter) Write(p []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return 0, context.DeadlineExceeded
	}
	return tw.buf.Write(p)
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return
	}
	tw.code = code
}

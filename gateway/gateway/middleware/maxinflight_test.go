package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithMaxInFlightRequests(t *testing.T) {
	tests := []struct {
		name         string
		maxInFlight  int
		handler      func(started chan<- struct{}, release <-chan struct{}) http.Handler
		sendRequests func(t *testing.T, serverURL string, started chan struct{}, release chan struct{})
	}{
		{
			name:        "disabled when maxInFlight is not positive",
			maxInFlight: 0,
			handler: func(_ chan<- struct{}, _ <-chan struct{}) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
			},
			sendRequests: func(t *testing.T, serverURL string, _ chan struct{}, _ chan struct{}) {
				t.Helper()
				resp, err := http.Get(serverURL)
				require.NoError(t, err)
				defer resp.Body.Close() //nolint:errcheck
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			},
		},
		{
			name:        "rejects when all slots are occupied",
			maxInFlight: 3,
			handler: func(started chan<- struct{}, release <-chan struct{}) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					started <- struct{}{}
					<-release
					w.WriteHeader(http.StatusOK)
				})
			},
			sendRequests: func(t *testing.T, serverURL string, started chan struct{}, release chan struct{}) {
				t.Helper()

				var wg sync.WaitGroup
				// Fill all 3 slots
				for range 3 {
					wg.Add(1)
					go func() {
						defer wg.Done()
						resp, err := http.Get(serverURL)
						require.NoError(t, err)
						defer resp.Body.Close() //nolint:errcheck //nolint:errcheck
						assert.Equal(t, http.StatusOK, resp.StatusCode)
					}()
					<-started
				}

				// 4th request should be rejected
				resp, err := http.Get(serverURL)
				require.NoError(t, err)
				defer resp.Body.Close() //nolint:errcheck
				assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

				close(release)
				wg.Wait()
			},
		},
		{
			name:        "slot released after handler returns",
			maxInFlight: 1,
			handler: func(_ chan<- struct{}, _ <-chan struct{}) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
			},
			sendRequests: func(t *testing.T, serverURL string, _ chan struct{}, _ chan struct{}) {
				t.Helper()
				for range 5 {
					resp, err := http.Get(serverURL)
					require.NoError(t, err)
					resp.Body.Close() //nolint:errcheck
					assert.Equal(t, http.StatusOK, resp.StatusCode)
				}
			},
		},
		{
			name:        "slot released after handler panics",
			maxInFlight: 1,
			handler: func(_ chan<- struct{}, _ <-chan struct{}) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer func() {
						if r := recover(); r != nil {
							w.WriteHeader(http.StatusInternalServerError)
						}
					}()
					panic("test panic")
				})
			},
			sendRequests: func(t *testing.T, serverURL string, _ chan struct{}, _ chan struct{}) {
				t.Helper()

				// First request panics
				resp, err := http.Get(serverURL)
				require.NoError(t, err)
				resp.Body.Close() //nolint:errcheck

				// Second request should succeed (slot was released)
				done := make(chan struct{})
				go func() {
					defer close(done)
					resp, err := http.Get(serverURL)
					require.NoError(t, err)
					resp.Body.Close() //nolint:errcheck
				}()

				select {
				case <-done:
					// success
				case <-time.After(2 * time.Second):
					t.Fatal("second request timed out — slot was not released after panic")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			started := make(chan struct{}, 10)
			release := make(chan struct{})

			handler := WithMaxInFlightRequests(tt.handler(started, release), tt.maxInFlight)
			server := httptest.NewServer(handler)
			defer server.Close()

			tt.sendRequests(t, server.URL, started, release)
		})
	}
}

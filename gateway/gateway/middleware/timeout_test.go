package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithTimeout(t *testing.T) {
	t.Run("handler completes before timeout", func(t *testing.T) {
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Custom", "value")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{}}`)) //nolint:errcheck
		})

		handler := WithTimeout(inner, 1*time.Second)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/graphql", nil)

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, `{"data":{}}`, rec.Body.String())
		assert.Equal(t, "value", rec.Header().Get("X-Custom"))
	})

	t.Run("handler exceeds timeout returns 504 with GraphQL JSON error", func(t *testing.T) {
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
			time.Sleep(10 * time.Millisecond) // ensure timeout path wins the select
		})

		handler := WithTimeout(inner, 10*time.Millisecond)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/graphql", nil)

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusGatewayTimeout, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		var resp struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Len(t, resp.Errors, 1)
		assert.Equal(t, "request timeout", resp.Errors[0].Message)
	})

	t.Run("zero timeout disables middleware", func(t *testing.T) {
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := WithTimeout(inner, 0)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/graphql", nil)

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("writes after timeout are discarded", func(t *testing.T) {
		writeDone := make(chan struct{})
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
			time.Sleep(20 * time.Millisecond) // let timeout response be written first
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("late write")) //nolint:errcheck
			close(writeDone)
		})

		handler := WithTimeout(inner, 10*time.Millisecond)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/graphql", nil)

		handler.ServeHTTP(rec, req)

		// Wait for the goroutine's late writes to complete
		<-writeDone

		// Response should be the timeout error, not the late write
		assert.Equal(t, http.StatusGatewayTimeout, rec.Code)
		assert.NotContains(t, rec.Body.String(), "late write")
	})
}

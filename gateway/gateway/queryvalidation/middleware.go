package queryvalidation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Middleware returns an http.Handler that validates incoming GraphQL queries
// against depth and complexity limits before forwarding to the next handler.
// Supports both single requests and batched query arrays.
// If all limits are zero, the middleware is a no-op passthrough.
func Middleware(next http.Handler, cfg Config) http.Handler {
	if cfg.MaxDepth <= 0 && cfg.MaxComplexity <= 0 && cfg.MaxBatchSize <= 0 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil || r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeGraphQLError(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		queries := extractQueries(body)
		if len(queries) == 0 {
			// Not valid JSON or no query field — let the GraphQL handler deal with it
			next.ServeHTTP(w, r)
			return
		}

		if cfg.MaxBatchSize > 0 && len(queries) > cfg.MaxBatchSize {
			writeGraphQLError(w, fmt.Sprintf("batch size %d exceeds maximum allowed batch size of %d", len(queries), cfg.MaxBatchSize), http.StatusBadRequest)
			return
		}

		for _, q := range queries {
			if err := Validate(q, cfg); err != nil {
				writeGraphQLError(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

type graphqlRequest struct {
	Query string `json:"query"`
}

// extractQueries extracts query strings from a GraphQL request body.
// Supports both single requests {"query":"..."} and batched requests [{"query":"..."}, ...].
// Returns the queries and true if extraction succeeded, or nil and false if the body
// is not valid JSON or contains no queries.
func extractQueries(body []byte) []string {
	var reqs []graphqlRequest
	if err := json.Unmarshal(body, &reqs); err != nil {
		var req graphqlRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil
		}
		reqs = []graphqlRequest{req}
	}

	queries := make([]string, 0, len(reqs))
	for _, req := range reqs {
		if req.Query != "" {
			queries = append(queries, req.Query)
		}
	}
	return queries
}

func writeGraphQLError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := map[string]any{
		"errors": []map[string]string{
			{"message": message},
		},
	}
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

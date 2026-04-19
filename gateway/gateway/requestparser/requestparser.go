package requestparser

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	utilscontext "github.com/platform-mesh/kubernetes-graphql-gateway/gateway/utils/context"
)

// Middleware reads the GraphQL request body, parses it into
// []utilscontext.GraphQLRequest, and stores the result in the request context.
// Downstream middlewares can call utilscontext.GetParsedRequestsFromCtx
// instead of re-parsing the body.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil || r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		if reqs := parseRequests(body); len(reqs) > 0 {
			ctx := utilscontext.SetParsedRequests(r.Context(), reqs)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

func parseRequests(body []byte) []utilscontext.GraphQLRequest {
	var reqs []utilscontext.GraphQLRequest
	if err := json.Unmarshal(body, &reqs); err == nil {
		return reqs
	}

	var req utilscontext.GraphQLRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil
	}
	return []utilscontext.GraphQLRequest{req}
}

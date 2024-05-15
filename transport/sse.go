package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/openmfp/crd-gql-gateway/gateway"
)

// New returns a new http.Handler that can serve a GraphQL subscription over SSE.
func New(schema graphql.Schema, userClaimName string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}

		claims := jwt.MapClaims{}
		_, _, err := jwt.NewParser().ParseUnverified(token, claims)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		userIdentifier, ok := claims[userClaimName].(string)
		if !ok || userIdentifier == "" {
			http.Error(w, "invalid user claim", http.StatusUnauthorized)
			return
		}

		ctx = gateway.AddUserToContext(ctx, userIdentifier)

		opts := handler.NewRequestOptions(r)

		rc := http.NewResponseController(w)
		defer rc.Flush()

		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Content-Type", "application/json")

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ":\n\n")
		rc.Flush()

		subscriptionChannel := graphql.Subscribe(graphql.Params{
			Context:        ctx,
			Schema:         schema,
			RequestString:  opts.Query,
			VariableValues: opts.Variables,
		})

		for result := range subscriptionChannel {
			b, _ := json.Marshal(result)
			fmt.Fprintf(w, "event: next\ndata: %s\n\n", b)
			rc.Flush()
		}

		fmt.Fprint(w, "event: complete\n\n")
	})
}

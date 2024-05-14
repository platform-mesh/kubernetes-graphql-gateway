package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
)

// New returns a new http.Handler that can serve a GraphQL subscription over SSE.
func New(schema graphql.Schema) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

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
			RootObject: map[string]interface{}{
				"token": strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "),
			},
		})

		for result := range subscriptionChannel {
			b, _ := json.Marshal(result)
			fmt.Fprintf(w, "event: next\ndata: %s\n\n", b)
			rc.Flush()
		}

		fmt.Fprint(w, "event: complete\n\n")
	})
}

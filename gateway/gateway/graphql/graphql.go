package graphql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

type GraphQLConfig struct {
	Pretty     bool
	Playground bool
	GraphiQL   bool
}

// GraphQLServer provides utility methods for creating GraphQL handlers
type GraphQLServer struct {
	config GraphQLConfig
}

// NewGraphQLServer creates a new GraphQL server
func NewGraphQLServer(config GraphQLConfig) *GraphQLServer {
	return &GraphQLServer{
		config: config,
	}
}

type GraphQLHandler struct {
	Schema  *graphql.Schema
	Handler http.Handler
}

// CreateHandler creates a new GraphQL handler from a schema
func (s *GraphQLServer) CreateHandler(schema *graphql.Schema) *GraphQLHandler {
	graphqlHandler := handler.New(&handler.Config{
		Schema:     schema,
		Pretty:     s.config.Pretty,
		Playground: s.config.Playground,
		GraphiQL:   s.config.GraphiQL,
	})
	return &GraphQLHandler{
		Schema:  schema,
		Handler: graphqlHandler,
	}
}

// IsIntrospectionQuery checks if the request contains a GraphQL introspection query
func IsIntrospectionQuery(r *http.Request) bool {
	var params struct {
		Query string `json:"query"`
	}
	bodyBytes, err := io.ReadAll(r.Body)
	r.Body.Close() //nolint:errcheck
	if err == nil {
		if err = json.Unmarshal(bodyBytes, &params); err == nil {
			if strings.Contains(params.Query, "__schema") || strings.Contains(params.Query, "__type") {
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return true
			}
		}
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return false
}

// HandleSubscription handles GraphQL subscription requests using Server-Sent Events
func (s *GraphQLServer) HandleSubscription(w http.ResponseWriter, r *http.Request, schema *graphql.Schema) {
	logger := log.FromContext(r.Context())

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var params struct {
		Query         string         `json:"query"`
		OperationName string         `json:"operationName"`
		Variables     map[string]any `json:"variables"`
	}

	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, "Error parsing JSON request body", http.StatusBadRequest)
		return
	}

	flusher := http.NewResponseController(w)
	r.Body.Close() //nolint:errcheck

	subscriptionParams := graphql.Params{
		Schema:         *schema,
		RequestString:  params.Query,
		VariableValues: params.Variables,
		OperationName:  params.OperationName,
		Context:        r.Context(),
	}

	subscriptionChannel := graphql.Subscribe(subscriptionParams)
	for res := range subscriptionChannel {
		if res == nil {
			continue
		}

		data, err := json.Marshal(res)
		if err != nil {
			logger.Error(err, "Error marshalling subscription response")
			continue
		}

		fmt.Fprintf(w, "event: next\ndata: %s\n\n", data) //nolint:errcheck
		flusher.Flush()                                   //nolint:errcheck
	}

	fmt.Fprint(w, "event: complete\n\n") //nolint:errcheck
}

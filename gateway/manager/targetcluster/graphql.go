package targetcluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/platform-mesh/golang-commons/logger"

	appConfig "github.com/platform-mesh/kubernetes-graphql-gateway/common/config"
	ctxkeys "github.com/platform-mesh/kubernetes-graphql-gateway/gateway/manager/context"
)

// WithClusterName stores a logical cluster name in the context
func WithClusterName(ctx context.Context, name logicalcluster.Name) context.Context {
	return ctxkeys.WithClusterName(ctx, name)
}

// ClusterNameFromContext retrieves a logical cluster name from the context
func ClusterNameFromContext(ctx context.Context) (logicalcluster.Name, bool) {
	return ctxkeys.ClusterNameFromContext(ctx)
}

// GraphQLHandler wraps a GraphQL schema and HTTP handler
type GraphQLHandler struct {
	Schema  *graphql.Schema
	Handler http.Handler
}

// GraphQLServer provides utility methods for creating GraphQL handlers
type GraphQLServer struct {
	log    *logger.Logger
	AppCfg appConfig.Config
}

// NewGraphQLServer creates a new GraphQL server
func NewGraphQLServer(log *logger.Logger, appCfg appConfig.Config) *GraphQLServer {
	return &GraphQLServer{
		log:    log,
		AppCfg: appCfg,
	}
}

// CreateHandler creates a new GraphQL handler from a schema
func (s *GraphQLServer) CreateHandler(schema *graphql.Schema) *GraphQLHandler {
	graphqlHandler := handler.New(&handler.Config{
		Schema:     schema,
		Pretty:     s.AppCfg.Gateway.HandlerCfg.Pretty,
		Playground: s.AppCfg.Gateway.HandlerCfg.Playground,
		GraphiQL:   s.AppCfg.Gateway.HandlerCfg.GraphiQL,
	})
	return &GraphQLHandler{
		Schema:  schema,
		Handler: graphqlHandler,
	}
}

// SetContexts sets the required contexts for KCP and authentication
func SetContexts(r *http.Request, workspace, token string, enableKcp bool) *http.Request {
	if enableKcp {
		// For virtual workspaces, the multicluster-runtime will handle cluster context
		// We just need to store the workspace name in the context for potential future use
		kcpWorkspaceName := workspace
		if kcpWorkspace, ok := ctxkeys.KcpWorkspaceFromContext(r.Context()); ok && kcpWorkspace != "" {
			kcpWorkspaceName = kcpWorkspace
		}
		// Store the logical cluster name in context using the helper function
		r = r.WithContext(WithClusterName(r.Context(), logicalcluster.Name(kcpWorkspaceName)))
	}
	return r.WithContext(ctxkeys.WithToken(r.Context(), token))
}

// GetToken extracts the token from the request Authorization header
func GetToken(r *http.Request) string {
	token := r.Header.Get("Authorization")
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimPrefix(token, "bearer ")
	return token
}

// IsIntrospectionQuery checks if the request contains a GraphQL introspection query
func IsIntrospectionQuery(r *http.Request) bool {
	var params struct {
		Query string `json:"query"`
	}
	bodyBytes, err := io.ReadAll(r.Body)
	r.Body.Close()
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
	r.Body.Close()

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
			s.log.Error().Err(err).Msg("Error marshalling subscription response")
			continue
		}

		fmt.Fprintf(w, "event: next\ndata: %s\n\n", data)
		flusher.Flush()
	}

	fmt.Fprint(w, "event: complete\n\n")
}

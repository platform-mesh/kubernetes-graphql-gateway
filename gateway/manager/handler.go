package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/kcp-dev/logicalcluster/v3"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/kontext"

	"github.com/openmfp/golang-commons/sentry"
)

var (
	ErrNoHandlerFound = errors.New("no handler found for workspace")
)

type handlerStore struct {
	mu       sync.RWMutex
	registry map[string]*graphqlHandler
}

type graphqlHandler struct {
	schema  *graphql.Schema
	handler http.Handler
}

func (s *Service) createHandler(schema *graphql.Schema) *graphqlHandler {
	h := handler.New(&handler.Config{
		Schema:     schema,
		Pretty:     s.AppCfg.Gateway.HandlerCfg.Pretty,
		Playground: s.AppCfg.Gateway.HandlerCfg.Playground,
		GraphiQL:   s.AppCfg.Gateway.HandlerCfg.GraphiQL,
	})
	return &graphqlHandler{
		schema:  schema,
		handler: h,
	}
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.handleCORS(w, r) {
		return
	}

	workspace, h, ok := s.getWorkspaceAndHandler(w, r)
	if !ok {
		return
	}

	if r.Method == http.MethodGet {
		h.handler.ServeHTTP(w, r)
		return
	}

	token := getToken(r)

	if !s.handleAuth(w, r, token) {
		return
	}

	r = s.setContexts(r, workspace, token)

	if r.Header.Get("Accept") == "text/event-stream" {
		s.handleSubscription(w, r, h.schema)
	} else {
		h.handler.ServeHTTP(w, r)
	}
}

func (s *Service) handleCORS(w http.ResponseWriter, r *http.Request) bool {
	if s.AppCfg.Gateway.Cors.Enabled {
		w.Header().Set("Access-Control-Allow-Origin", s.AppCfg.Gateway.Cors.AllowedOrigins)
		w.Header().Set("Access-Control-Allow-Headers", s.AppCfg.Gateway.Cors.AllowedHeaders)
		// setting cors allowed methods is not needed for this service,
		// as all graphql methods are part of the cors safelisted methods
		// https://fetch.spec.whatwg.org/#cors-safelisted-method

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return true
		}
	}
	return false
}

// getWorkspaceAndHandler extracts the workspace from the path, finds the handler, and handles errors.
// Returns workspace, handler, and ok (true if found, false if error was handled).
func (s *Service) getWorkspaceAndHandler(w http.ResponseWriter, r *http.Request) (string, *graphqlHandler, bool) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 2 {
		s.log.Error().Err(fmt.Errorf("invalid path")).Str("path", r.URL.Path).Msg("Error parsing path")
		http.NotFound(w, r)
		return "", nil, false
	}

	workspace := parts[0]

	s.handlers.mu.RLock()
	h, ok := s.handlers.registry[workspace]
	s.handlers.mu.RUnlock()

	if !ok {
		s.log.Error().Err(ErrNoHandlerFound).Str("workspace", workspace)
		sentry.CaptureError(ErrNoHandlerFound, sentry.Tags{"workspace": workspace})
		http.NotFound(w, r)
		return "", nil, false
	}

	return workspace, h, true
}

func getToken(r *http.Request) string {
	token := r.Header.Get("Authorization")
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimPrefix(token, "bearer ")

	return token
}

func (s *Service) handleAuth(w http.ResponseWriter, r *http.Request, token string) bool {
	if !s.AppCfg.LocalDevelopment {
		if token == "" {
			http.Error(w, "Authorization header is required", http.StatusUnauthorized)
			return false
		}

		if s.AppCfg.IntrospectionAuthentication {
			if s.isIntrospectionQuery(r) {
				ok, err := s.validateToken(r.Context(), token)
				if err != nil {
					s.log.Error().Err(err).Msg("error validating token with k8s")
					http.Error(w, "error validating token", http.StatusInternalServerError)
					return false
				}

				if !ok {
					http.Error(w, "Provided token is not authorized to access the cluster", http.StatusUnauthorized)
					return false
				}
			}
		}
	}
	return true
}

func (s *Service) isIntrospectionQuery(r *http.Request) bool {
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

// validateToken uses the /version endpoint for a general authentication check.
func (s *Service) validateToken(ctx context.Context, token string) (bool, error) {
	cfg := &rest.Config{
		Host: s.restCfg.Host,
		TLSClientConfig: rest.TLSClientConfig{
			CAFile: s.restCfg.TLSClientConfig.CAFile,
			CAData: s.restCfg.TLSClientConfig.CAData,
		},
		BearerToken: token,
	}

	httpClient, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/version", cfg.Host), nil)
	if err != nil {
		return false, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return false, nil
	case http.StatusOK:
		return true, nil
	default:
		return false, fmt.Errorf("unexpected status code from /version: %d", resp.StatusCode)
	}
}

func (s *Service) setContexts(r *http.Request, workspace, token string) *http.Request {
	if s.AppCfg.EnableKcp {
		r = r.WithContext(kontext.WithCluster(r.Context(), logicalcluster.Name(workspace)))
	}
	return r.WithContext(context.WithValue(r.Context(), TokenKey{}, token))
}

func (s *Service) handleSubscription(w http.ResponseWriter, r *http.Request, schema *graphql.Schema) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var params struct {
		Query         string                 `json:"query"`
		OperationName string                 `json:"operationName"`
		Variables     map[string]interface{} `json:"variables"`
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

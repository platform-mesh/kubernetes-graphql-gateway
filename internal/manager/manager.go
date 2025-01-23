package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/go-openapi/spec"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/kcp-dev/logicalcluster/v3"
	appConfig "github.com/openmfp/crd-gql-gateway/internal/config"
	"github.com/openmfp/crd-gql-gateway/internal/gateway"
	"github.com/openmfp/crd-gql-gateway/internal/resolver"
	"github.com/openmfp/golang-commons/logger"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/kcp"
	"sigs.k8s.io/controller-runtime/pkg/kontext"
)

type Provider interface {
	Start()
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type FileWatcher interface {
	OnFileChanged(filename string)
	OnFileDeleted(filename string)
}

type Service struct {
	appCfg   appConfig.Config
	restCfg  *rest.Config
	log      *logger.Logger
	resolver resolver.Provider
	handlers map[string]*graphqlHandler
	mu       sync.RWMutex
	watcher  *fsnotify.Watcher
}

type graphqlHandler struct {
	schema  *graphql.Schema
	handler http.Handler
}

func NewManager(log *logger.Logger, cfg *rest.Config, appCfg appConfig.Config) (*Service, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	m := &Service{
		appCfg:   appCfg,
		restCfg:  cfg,
		log:      log,
		resolver: resolver.New(log),
		handlers: make(map[string]*graphqlHandler),
		watcher:  watcher,
	}

	err = m.watcher.Add(appCfg.WatchedDir)
	if err != nil {
		return nil, err
	}

	files, err := filepath.Glob(filepath.Join(appCfg.WatchedDir, "*"))
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		filename := filepath.Base(file)
		m.OnFileChanged(filename)
	}

	m.Start()

	return m, nil
}

func (s *Service) Start() {
	go func() {
		for {
			select {
			case event, ok := <-s.watcher.Events:
				if !ok {
					return
				}
				s.handleEvent(event)
			case err, ok := <-s.watcher.Errors:
				if !ok {
					return
				}
				s.log.Error().Err(err).Msg("Error watching files")
			}
		}
	}()
}

func (s *Service) handleEvent(event fsnotify.Event) {
	s.log.Info().Str("event", event.String()).Msg("File event")

	filename := filepath.Base(event.Name)
	switch event.Op {
	case fsnotify.Create:
		s.OnFileChanged(filename)
	case fsnotify.Write:
		s.OnFileChanged(filename)
	case fsnotify.Rename:
		s.OnFileDeleted(filename)
	case fsnotify.Remove:
		s.OnFileDeleted(filename)
	default:
		s.log.Info().Str("file", filename).Msg("Unknown file event")
	}
}

func (s *Service) OnFileChanged(filename string) {
	schema, err := s.loadSchemaFromFile(filename)
	if err != nil {
		s.log.Error().Err(err).Str("file", filename).Msg("Error loading example:alpha from file")
		return
	}

	s.mu.Lock()
	s.handlers[filename] = s.createHandler(schema)
	s.mu.Unlock()

	s.log.Info().Str("endpoint", fmt.Sprintf("http://localhost:%s/%s/graphql", s.appCfg.Port, filename)).Msg("Registered endpoint")
}

func (s *Service) OnFileDeleted(filename string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.handlers, filename)

}

func (s *Service) loadSchemaFromFile(filename string) (*graphql.Schema, error) {
	definitions, err := readDefinitionFromFile(filepath.Join(s.appCfg.WatchedDir, filename))
	if err != nil {
		return nil, err
	}

	g, err := gateway.New(s.log, definitions, s.resolver)
	if err != nil {
		return nil, err
	}

	return g.GetSchema(), nil
}

func (s *Service) createHandler(schema *graphql.Schema) *graphqlHandler {
	h := handler.New(&handler.Config{
		Schema:     schema,
		Pretty:     s.appCfg.HandlerCfg.Pretty,
		Playground: s.appCfg.HandlerCfg.Playground,
		GraphiQL:   s.appCfg.HandlerCfg.GraphiQL,
	})
	return &graphqlHandler{
		schema:  schema,
		handler: h,
	}
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	workspace, endpoint, err := s.parsePath(r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	h, ok := s.getHandler(workspace)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if endpoint == "graphql" && r.Method == http.MethodGet {
		h.handler.ServeHTTP(w, r)
		return
	}

	cfg, err := s.getConfigForRuntimeClient(workspace, r.Header.Get("Authorization"))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Error getting a runtime client's config")
		return
	}

	r = r.WithContext(kontext.WithCluster(r.Context(), logicalcluster.Name(workspace)))

	runtimeClient, err := setupK8sClients(r.Context(), cfg)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Error setting up Kubernetes client")
		return
	}

	r = r.WithContext(context.WithValue(r.Context(), resolver.RuntimeClientKey{}, runtimeClient))

	if r.Header.Get("Accept") == "text/event-stream" {
		s.handleSubscription(w, r, h.schema)
	} else {
		h.handler.ServeHTTP(w, r)
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(map[string]interface{}{
		"errors": []map[string]string{
			{"message": message},
		},
	})
	if err != nil {
		http.Error(w, "Error writing JSON response", http.StatusInternalServerError)
	}
}

// parsePath extracts filename and endpoint from the requested URL path.
func (s *Service) parsePath(path string) (workspace, endpoint string, err error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid path")
	}
	return parts[0], parts[1], nil
}

// getHandler retrieves the graphqlHandler associated with the given filename.
func (s *Service) getHandler(filename string) (*graphqlHandler, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, ok := s.handlers[filename]
	return h, ok
}

// getConfigForRuntimeClient initializes a runtime client for the given server address.
func (s *Service) getConfigForRuntimeClient(workspace, token string) (*rest.Config, error) {
	if token == "" { // if no token, use current-context
		return s.restCfg, nil
	}

	requestConfig := rest.CopyConfig(s.restCfg)
	requestConfig.BearerToken = token
	u, err := url.Parse(s.restCfg.Host)
	if err != nil {
		return nil, err
	}

	base := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	requestConfig.Host = base
	requestConfig.APIPath = fmt.Sprintf("/clusters/%s", workspace)

	return requestConfig, nil
}

func (s *Service) handleSubscription(w http.ResponseWriter, r *http.Request, schema *graphql.Schema) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher := http.NewResponseController(w)

	var params struct {
		Query         string                 `json:"query"`
		OperationName string                 `json:"operationName"`
		Variables     map[string]interface{} `json:"variables"`
	}

	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, "Error parsing JSON request body", http.StatusBadRequest)
		return
	}

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
			continue
		}
		fmt.Fprintf(w, "event: next\ndata: %s\n\n", data)
		flusher.Flush()
	}

	fmt.Fprint(w, "event: complete\n\n")
}

func readDefinitionFromFile(filePath string) (spec.Definitions, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var swagger spec.Swagger
	err = json.Unmarshal(data, &swagger)
	if err != nil {
		return nil, err
	}

	return swagger.Definitions, nil
}

// setupK8sClients initializes and returns the runtime client for Kubernetes.
func setupK8sClients(ctx context.Context, cfg *rest.Config) (client.WithWatch, error) {
	opts := client.Options{
		Scheme: scheme.Scheme,
	}

	if cluster, ok := kontext.ClusterFrom(ctx); ok && !cluster.Empty() {
		httpClient, err := kcp.NewClusterAwareHTTPClient(cfg)
		if err != nil {
			return nil, err
		}

		opts.HTTPClient = httpClient
		opts.MapperWithContext = func(ctx context.Context) (meta.RESTMapper, error) {
			return kcp.NewClusterAwareMapperProvider(cfg, httpClient)(ctx)
		}
	}

	runtimeClient, err := client.NewWithWatch(cfg, opts)

	return runtimeClient, err
}

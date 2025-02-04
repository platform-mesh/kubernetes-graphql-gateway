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
	appConfig "github.com/openmfp/crd-gql-gateway/gateway/config"
	"github.com/openmfp/crd-gql-gateway/gateway/resolver"
	"github.com/openmfp/crd-gql-gateway/gateway/schema"
	"github.com/openmfp/golang-commons/logger"
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
	handlers map[string]*graphqlHandler
	log      *logger.Logger
	mu       sync.RWMutex
	resolver resolver.Provider
	restCfg  *rest.Config
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

	// lets ensure that kcp url points directly to kcp domain
	u, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, err
	}
	cfg.Host = fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	cfg.Wrap(func(rt http.RoundTripper) http.RoundTripper {
		return NewRoundTripper(log, rt)
	})

	runtimeClient, err := kcp.NewClusterAwareClientWithWatch(cfg, client.Options{})
	if err != nil {
		return nil, err
	}

	m := &Service{
		appCfg:   appCfg,
		handlers: make(map[string]*graphqlHandler),
		log:      log,
		resolver: resolver.New(log, runtimeClient),
		restCfg:  cfg,
		watcher:  watcher,
	}

	err = m.watcher.Add(appCfg.OpenApiDefinitionsPath)
	if err != nil {
		return nil, err
	}

	files, err := filepath.Glob(filepath.Join(appCfg.OpenApiDefinitionsPath, "*"))
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
		s.log.Error().Err(err).Str("file", filename).Msg("Error loading schema from file")
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
	definitions, err := readDefinitionFromFile(filepath.Join(s.appCfg.OpenApiDefinitionsPath, filename))
	if err != nil {
		return nil, err
	}

	g, err := schema.New(s.log, definitions, s.resolver)
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
	workspace, err := s.parsePath(r.URL.Path)
	if err != nil {
		s.log.Error().Err(err).Str("path", r.URL.Path).Msg("Error parsing path")
		http.NotFound(w, r)
		return
	}

	s.mu.RLock()
	h, ok := s.handlers[workspace]
	s.mu.RUnlock()

	if !ok {
		s.log.Info().Str("workspace", workspace).Msg("no handler found for workspace")
		http.NotFound(w, r)
		return
	}

	if r.Method == http.MethodGet {
		h.handler.ServeHTTP(w, r)
		return
	}

	token := r.Header.Get("Authorization")
	if !s.appCfg.LocalDevelopment && token == "" {
		http.Error(w, "Authorization header is required", http.StatusUnauthorized)
		return
	}

	if s.appCfg.EnableKcp {
		r = r.WithContext(kontext.WithCluster(r.Context(), logicalcluster.Name(workspace)))
	}

	split := strings.Split(token, " ")
	if len(split) == 1 {
		r = r.WithContext(context.WithValue(r.Context(), TokenKey{}, token))
	} else {
		r = r.WithContext(context.WithValue(r.Context(), TokenKey{}, split[1]))
	}

	if r.Header.Get("Accept") == "text/event-stream" {
		s.handleSubscription(w, r, h.schema)
	} else {
		h.handler.ServeHTTP(w, r)
	}
}

// parsePath extracts filename and endpoint from the requested URL path.
func (s *Service) parsePath(path string) (workspace string, err error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid path")
	}

	return parts[0], nil
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

func readDefinitionFromFile(filePath string) (spec.Definitions, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var swagger spec.Swagger
	err = json.NewDecoder(f).Decode(&swagger)
	if err != nil {
		return nil, err
	}

	return swagger.Definitions, nil
}

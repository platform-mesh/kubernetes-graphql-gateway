package manager

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/openmfp/golang-commons/logger"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/kcp"

	appConfig "github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/resolver"
)

type Provider interface {
	Start()
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type Service struct {
	AppCfg  appConfig.Config
	restCfg *rest.Config

	log      *logger.Logger
	resolver resolver.Provider

	handlers handlerStore
	watcher  *fsnotify.Watcher
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
		return NewRoundTripper(log, rt, appCfg.Gateway.UsernameClaim, appCfg.Gateway.ShouldImpersonate)
	})

	runtimeClient, err := kcp.NewClusterAwareClientWithWatch(cfg, client.Options{})
	if err != nil {
		return nil, err
	}

	m := &Service{
		AppCfg: appCfg,
		handlers: handlerStore{
			registry: make(map[string]*graphqlHandler),
		},
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

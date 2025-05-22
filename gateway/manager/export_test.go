package manager

import (
	"github.com/openmfp/golang-commons/logger/testlogger"
	appConfig "github.com/openmfp/kubernetes-graphql-gateway/common/config"
)

func NewManagerForTest() *Service {
	cfg := appConfig.Config{}
	cfg.Gateway.Cors.Enabled = true
	cfg.Gateway.Cors.AllowedOrigins = []string{"*"}
	cfg.Gateway.Cors.AllowedHeaders = []string{"Authorization"}

	s := &Service{
		AppCfg:   cfg,
		handlers: handlerStore{registry: make(map[string]*graphqlHandler)},
		log:      testlogger.New().HideLogOutput().Logger,
		resolver: nil,
	}
	s.handlers.registry["testws"] = &graphqlHandler{}

	return s
}

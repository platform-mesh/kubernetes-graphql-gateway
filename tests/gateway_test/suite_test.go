package gateway_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/openmfp/golang-commons/logger"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/kcp"

	appConfig "github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/schema"
)

type CommonTestSuite struct {
	suite.Suite
	testEnv       *envtest.Environment
	log           *logger.Logger
	restCfg       *rest.Config
	appCfg        appConfig.Config
	runtimeClient client.WithWatch
	schema        graphql.Schema
	manager       manager.Provider
	server        *httptest.Server
}

func TestCommonTestSuite(t *testing.T) {
	suite.Run(t, new(CommonTestSuite))
}

func (suite *CommonTestSuite) SetupTest() {
	var err error
	suite.testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			// this is needed for the CRD registration
			filepath.Join("testdata", "crd"),
		},
	}
	suite.restCfg, err = suite.testEnv.Start()
	require.NoError(suite.T(), err)

	suite.appCfg.OpenApiDefinitionsPath, err = os.MkdirTemp("", "watchedDir")
	require.NoError(suite.T(), err)

	suite.appCfg.LocalDevelopment = true

	suite.log, err = logger.New(logger.DefaultConfig())
	require.NoError(suite.T(), err)

	suite.runtimeClient, err = kcp.NewClusterAwareClientWithWatch(suite.restCfg, client.Options{})
	require.NoError(suite.T(), err)

	definitions, err := manager.ReadDefinitionFromFile("./testdata/kubernetes")
	require.NoError(suite.T(), err)

	g, err := schema.New(suite.log, definitions, resolver.New(suite.log, suite.runtimeClient))
	require.NoError(suite.T(), err)

	suite.schema = *g.GetSchema()

	suite.manager, err = manager.NewManager(suite.log, suite.restCfg, suite.appCfg)
	require.NoError(suite.T(), err)

	suite.server = httptest.NewServer(suite.manager)
}

func (suite *CommonTestSuite) TearDownTest() {
	require.NoError(suite.T(), os.RemoveAll(suite.appCfg.OpenApiDefinitionsPath))
	require.NoError(suite.T(), suite.testEnv.Stop())
	suite.server.Close()
}

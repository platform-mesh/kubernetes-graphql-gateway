package gateway

import (
	"github.com/graphql-go/graphql"
	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/schema"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/rest"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/kcp"
	"testing"
)

type CommonTestSuite struct {
	suite.Suite
	testEnv       *envtest.Environment
	log           *logger.Logger
	restCfg       *rest.Config
	runtimeClient client.WithWatch
	schema        graphql.Schema
}

func TestCommonTestSuite(t *testing.T) {
	suite.Run(t, new(CommonTestSuite))
}

func (suite *CommonTestSuite) SetupTest() {
	var err error
	suite.testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("testdata", "crd"),
		},
	}
	suite.restCfg, err = suite.testEnv.Start()
	require.NoError(suite.T(), err)

	suite.log, err = logger.New(logger.DefaultConfig())
	require.NoError(suite.T(), err)

	suite.runtimeClient, err = kcp.NewClusterAwareClientWithWatch(suite.restCfg, client.Options{})
	require.NoError(suite.T(), err)

	definitions, err := manager.ReadDefinitionFromFile("./testdata/kubernetes")
	require.NoError(suite.T(), err)

	g, err := schema.New(suite.log, definitions, resolver.New(suite.log, suite.runtimeClient))
	require.NoError(suite.T(), err)

	suite.schema = *g.GetSchema()
}

func (suite *CommonTestSuite) TearDownTest() {
	require.NoError(suite.T(), suite.testEnv.Stop())
}
